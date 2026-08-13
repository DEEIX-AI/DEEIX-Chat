import * as React from "react";

type BrowserSpeechRecognitionAlternative = {
  transcript: string;
};

type BrowserSpeechRecognitionResult = {
  isFinal: boolean;
  length: number;
  item: (index: number) => BrowserSpeechRecognitionAlternative;
};

type BrowserSpeechRecognitionResultList = {
  length: number;
  item: (index: number) => BrowserSpeechRecognitionResult;
};

type BrowserSpeechRecognitionEvent = Event & {
  results: BrowserSpeechRecognitionResultList;
};

type BrowserSpeechRecognitionErrorEvent = Event & {
  error: string;
};

type BrowserSpeechRecognition = EventTarget & {
  continuous: boolean;
  interimResults: boolean;
  lang: string;
  onend: (() => void) | null;
  onerror: ((event: BrowserSpeechRecognitionErrorEvent) => void) | null;
  onresult: ((event: BrowserSpeechRecognitionEvent) => void) | null;
  onstart: (() => void) | null;
  start: () => void;
  stop: () => void;
};

type BrowserSpeechRecognitionConstructor = new () => BrowserSpeechRecognition;

type BrowserWindowWithSpeechRecognition = Window & {
  SpeechRecognition?: BrowserSpeechRecognitionConstructor;
  webkitSpeechRecognition?: BrowserSpeechRecognitionConstructor;
};

export type SpeechInputStatus = "idle" | "starting" | "listening";

export type SpeechInputErrorCode =
  | "audioUnavailable"
  | "interrupted"
  | "languageUnsupported"
  | "network"
  | "noSpeech"
  | "permissionDenied"
  | "serviceUnavailable"
  | "startFailed"
  | "unavailable";

const MAX_EMPTY_RESTARTS = 2;
const RESTART_DELAY_MS = 250;
const SPEECH_RECOGNITION_ERROR_CODES: Readonly<Record<string, SpeechInputErrorCode>> = {
  "audio-capture": "audioUnavailable",
  "language-not-supported": "languageUnsupported",
  network: "network",
  "not-allowed": "permissionDenied",
  "service-not-allowed": "serviceUnavailable",
};

type UseChatSpeechInputParams = {
  draft: string;
  language: string;
  listeningPlaceholder: string;
  onDraftChange: (value: string) => void;
  onError: (error: SpeechInputErrorCode) => void;
  placeholder: string;
  startingPlaceholder: string;
};

type UseChatSpeechInputState = {
  supported: boolean;
  status: SpeechInputStatus;
  active: boolean;
  placeholder: string;
  toggle: () => void;
};

export function useChatSpeechInput({
  draft,
  language,
  listeningPlaceholder,
  onDraftChange,
  onError,
  placeholder,
  startingPlaceholder,
}: UseChatSpeechInputParams): UseChatSpeechInputState {
  const [supported, setSupported] = React.useState(false);
  const [status, setStatus] = React.useState<SpeechInputStatus>("idle");
  const recognitionRef = React.useRef<BrowserSpeechRecognition | null>(null);
  const draftRef = React.useRef(draft);
  const baseDraftRef = React.useRef("");
  const renderedDraftRef = React.useRef("");
  const cancelledRef = React.useRef(false);
  const emptyRestartCountRef = React.useRef(0);
  const sessionHadResultRef = React.useRef(false);
  const recoverableErrorRef = React.useRef<"aborted" | "no-speech" | null>(null);
  const restartTimerRef = React.useRef<number | null>(null);

  const active = status !== "idle";
  const resolvedPlaceholder = status === "starting"
    ? startingPlaceholder
    : status === "listening"
      ? listeningPlaceholder
      : placeholder;

  React.useEffect(() => {
    draftRef.current = draft;
  }, [draft]);

  React.useEffect(() => {
    const browserWindow = window as BrowserWindowWithSpeechRecognition;
    const RecognitionConstructor = browserWindow.SpeechRecognition ?? browserWindow.webkitSpeechRecognition;
    setSupported(window.isSecureContext && Boolean(RecognitionConstructor));

    return () => {
      if (restartTimerRef.current !== null) {
        window.clearTimeout(restartTimerRef.current);
        restartTimerRef.current = null;
      }
      cancelledRef.current = true;
      const recognition = recognitionRef.current;
      recognitionRef.current = null;
      recognition?.stop();
    };
  }, []);

  const commitTranscript = React.useCallback(
    (finalTranscript: string, interimTranscript: string) => {
      const nextDraft = [
        baseDraftRef.current,
        finalTranscript.trim(),
        interimTranscript.trim(),
      ].filter(Boolean).join(" ");
      renderedDraftRef.current = nextDraft;
      draftRef.current = nextDraft;
      onDraftChange(nextDraft);
    },
    [onDraftChange],
  );

  const stop = React.useCallback(() => {
    cancelledRef.current = true;
    if (restartTimerRef.current !== null) {
      window.clearTimeout(restartTimerRef.current);
      restartTimerRef.current = null;
    }
    const recognition = recognitionRef.current;
    recognitionRef.current = null;
    recognition?.stop();
    setStatus("idle");
  }, []);

  const toggle = React.useCallback(() => {
    if (!supported) {
      onError("unavailable");
      return;
    }
    if (active) {
      stop();
      return;
    }

    const browserWindow = window as BrowserWindowWithSpeechRecognition;
    const RecognitionConstructor = browserWindow.SpeechRecognition ?? browserWindow.webkitSpeechRecognition;
    if (!RecognitionConstructor) {
      setSupported(false);
      onError("unavailable");
      return;
    }

    cancelledRef.current = false;
    emptyRestartCountRef.current = 0;
    baseDraftRef.current = draftRef.current.trimEnd();
    renderedDraftRef.current = baseDraftRef.current;

    const failStart = () => {
      cancelledRef.current = true;
      recognitionRef.current = null;
      setStatus("idle");
      onError("startFailed");
    };

    const startRecognition = () => {
      let recognition: BrowserSpeechRecognition;
      try {
        recognition = new RecognitionConstructor();
      } catch {
        failStart();
        return;
      }

      sessionHadResultRef.current = false;
      recoverableErrorRef.current = null;
      recognition.continuous = false;
      recognition.interimResults = true;
      recognition.lang = language;

      const finishWithError = (error: SpeechInputErrorCode) => {
        if (recognitionRef.current !== recognition) {
          return;
        }
        cancelledRef.current = true;
        recognitionRef.current = null;
        setStatus("idle");
        onError(error);
      };

      recognition.onstart = () => {
        if (!cancelledRef.current && recognitionRef.current === recognition) {
          setStatus("listening");
        }
      };
      recognition.onresult = (event) => {
        if (cancelledRef.current || recognitionRef.current !== recognition) {
          return;
        }

        const finalTranscripts: string[] = [];
        const interimTranscripts: string[] = [];
        for (let resultIndex = 0; resultIndex < event.results.length; resultIndex += 1) {
          const result = event.results.item(resultIndex);
          if (result.length === 0) {
            continue;
          }
          const transcript = result.item(0).transcript.trim();
          if (!transcript) {
            continue;
          }
          if (result.isFinal) {
            finalTranscripts.push(transcript);
          } else {
            interimTranscripts.push(transcript);
          }
        }
        if (finalTranscripts.length === 0 && interimTranscripts.length === 0) {
          return;
        }

        sessionHadResultRef.current = true;
        emptyRestartCountRef.current = 0;
        recoverableErrorRef.current = null;
        setStatus("listening");
        commitTranscript(finalTranscripts.join(" "), interimTranscripts.join(" "));
      };
      recognition.onerror = (event) => {
        if (cancelledRef.current || recognitionRef.current !== recognition) {
          return;
        }
        if (event.error === "no-speech" || event.error === "aborted") {
          recoverableErrorRef.current = event.error;
          setStatus("starting");
          return;
        }

        finishWithError(SPEECH_RECOGNITION_ERROR_CODES[event.error] ?? "unavailable");
      };
      recognition.onend = () => {
        if (recognitionRef.current !== recognition) {
          return;
        }
        if (cancelledRef.current) {
          recognitionRef.current = null;
          setStatus("idle");
          return;
        }

        baseDraftRef.current = renderedDraftRef.current.trimEnd();
        if (!sessionHadResultRef.current) {
          emptyRestartCountRef.current += 1;
          if (emptyRestartCountRef.current > MAX_EMPTY_RESTARTS) {
            finishWithError(recoverableErrorRef.current === "aborted" ? "interrupted" : "noSpeech");
            return;
          }
        }

        setStatus("starting");
        restartTimerRef.current = window.setTimeout(() => {
          restartTimerRef.current = null;
          if (cancelledRef.current || recognitionRef.current !== recognition) {
            return;
          }
          startRecognition();
        }, RESTART_DELAY_MS);
      };

      recognitionRef.current = recognition;
      setStatus("starting");
      try {
        recognition.start();
      } catch {
        failStart();
      }
    };

    startRecognition();
  }, [active, commitTranscript, language, onError, stop, supported]);

  return {
    supported,
    status,
    active,
    placeholder: resolvedPlaceholder,
    toggle,
  };
}
