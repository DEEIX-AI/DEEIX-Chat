// pdfjs-dist's build entry ships without type declarations; reuse the main package types.
declare module "pdfjs-dist/build/pdf.mjs" {
  export * from "pdfjs-dist";
}
