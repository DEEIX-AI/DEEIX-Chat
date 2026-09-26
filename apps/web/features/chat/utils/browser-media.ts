export function captureScreenshotFile(stream: MediaStream): Promise<File> {
  return new Promise((resolve, reject) => {
    const video = document.createElement("video");
    video.srcObject = stream;
    video.muted = true;
    video.playsInline = true;

    const cleanup = () => {
      video.pause();
      video.srcObject = null;
    };

    video.onerror = () => {
      cleanup();
      reject(new Error("Unable to read the screen stream"));
    };

    video.onloadedmetadata = () => {
      void video
        .play()
        .then(() => {
          const width = video.videoWidth;
          const height = video.videoHeight;
          if (width <= 0 || height <= 0) {
            cleanup();
            reject(new Error("Invalid screenshot size"));
            return;
          }

          const canvas = document.createElement("canvas");
          canvas.width = width;
          canvas.height = height;
          const context = canvas.getContext("2d");
          if (!context) {
            cleanup();
            reject(new Error("Screenshot failed"));
            return;
          }

          context.drawImage(video, 0, 0, width, height);
          canvas.toBlob(
            (blob) => {
              cleanup();
              if (!blob) {
                reject(new Error("Screenshot failed"));
                return;
              }
              const suffix = new Date().toISOString().replace(/[:.]/g, "-");
              resolve(new File([blob], `screenshot-${suffix}.png`, { type: "image/png" }));
            },
            "image/png",
            1,
          );
        })
        .catch((error: unknown) => {
          cleanup();
          reject(error instanceof Error ? error : new Error("Screenshot failed"));
        });
    };
  });
}
