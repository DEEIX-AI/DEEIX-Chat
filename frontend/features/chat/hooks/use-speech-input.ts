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
  resultIndex: number;
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

export type SpeechInputStatus = "idle" | "listening";

type UseSpeechInputParams = {
  draft: string;
  listeningPlaceholder: string;
  onDraftChange: (value: string) => void;
  placeholder: string;
};

type UseSpeechInputState = {
  supported: boolean;
  status: SpeechInputStatus;
  active: boolean;
  placeholder: string;
  toggle: () => void;
};

export function useSpeechInput({
  draft,
  listeningPlaceholder,
  onDraftChange,
  placeholder,
}: UseSpeechInputParams): UseSpeechInputState {
  const [supported, setSupported] = React.useState(false);
  const [status, setStatus] = React.useState<SpeechInputStatus>("idle");
  const recognitionRef = React.useRef<BrowserSpeechRecognition | null>(null);
  const draftRef = React.useRef(draft);
  const baseDraftRef = React.useRef("");
  const cancelledRef = React.useRef(false);
  const restartTimerRef = React.useRef<number | null>(null);

  const active = status !== "idle";
  const resolvedPlaceholder = active ? listeningPlaceholder : placeholder;

  React.useEffect(() => {
    draftRef.current = draft;
  }, [draft]);

  React.useEffect(() => {
    const browserWindow = window as BrowserWindowWithSpeechRecognition;
    setSupported(Boolean(browserWindow.SpeechRecognition ?? browserWindow.webkitSpeechRecognition));

    return () => {
      if (restartTimerRef.current !== null) {
        window.clearTimeout(restartTimerRef.current);
        restartTimerRef.current = null;
      }
      cancelledRef.current = true;
      recognitionRef.current?.stop();
      recognitionRef.current = null;
    };
  }, []);

  const commitTranscript = React.useCallback(
    (finalTranscript: string, interimTranscript: string) => {
      const fragments = [
        baseDraftRef.current,
        finalTranscript.trim(),
        interimTranscript.trim(),
      ].filter(Boolean);
      onDraftChange(fragments.join(" "));
    },
    [onDraftChange],
  );

  const stop = React.useCallback(() => {
    cancelledRef.current = true;
    if (restartTimerRef.current !== null) {
      window.clearTimeout(restartTimerRef.current);
      restartTimerRef.current = null;
    }
    recognitionRef.current?.stop();
    setStatus("idle");
  }, []);

  const toggle = React.useCallback(() => {
    if (!supported) {
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
      return;
    }

    cancelledRef.current = false;
    baseDraftRef.current = draftRef.current.trimEnd();

    const startRecognition = () => {
      const recognition = new RecognitionConstructor();
      recognition.continuous = true;
      recognition.interimResults = true;
      recognition.lang = navigator.language || "zh-CN";
      recognition.onstart = () => {
        setStatus("listening");
      };
      recognition.onresult = (event) => {
        setStatus("listening");
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
        commitTranscript(finalTranscripts.join(" "), interimTranscripts.join(" "));
      };
      recognition.onerror = (event) => {
        if (cancelledRef.current) {
          setStatus("idle");
          return;
        }
        if (event.error === "no-speech" || event.error === "aborted") {
          setStatus("listening");
          return;
        }
        cancelledRef.current = true;
        recognitionRef.current = null;
        setStatus("idle");
      };
      recognition.onend = () => {
        if (!cancelledRef.current) {
          if (recognitionRef.current !== recognition) {
            return;
          }
          setStatus("listening");
          restartTimerRef.current = window.setTimeout(() => {
            restartTimerRef.current = null;
            if (cancelledRef.current || recognitionRef.current !== recognition) {
              return;
            }
            startRecognition();
          }, 180);
          return;
        }
        if (recognitionRef.current === recognition) {
          recognitionRef.current = null;
        }
        setStatus("idle");
      };

      recognitionRef.current = recognition;
      try {
        recognition.start();
        setStatus("listening");
      } catch {
        recognitionRef.current = null;
        setStatus("idle");
      }
    };

    startRecognition();
  }, [active, commitTranscript, stop, supported]);

  return {
    supported,
    status,
    active,
    placeholder: resolvedPlaceholder,
    toggle,
  };
}
