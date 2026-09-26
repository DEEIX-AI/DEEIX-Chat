// pdfjs-dist 的 build 产物入口没有随包声明文件，复用主包类型。
declare module "pdfjs-dist/build/pdf.mjs" {
  export * from "pdfjs-dist";
}
