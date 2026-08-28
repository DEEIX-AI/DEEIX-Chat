// Computed styles required to preserve chat layout and visual fidelity in the SVG clone.
// Interaction-only, pagination, multi-column and print properties are intentionally excluded.
const SCREENSHOT_STYLE_PROPERTY_GROUPS = [
  "-webkit-background-clip -webkit-box-orient -webkit-line-clamp -webkit-mask-image -webkit-mask-position -webkit-mask-repeat -webkit-mask-size -webkit-text-fill-color",
  "display position top right bottom left z-index box-sizing width min-width max-width height min-height max-height aspect-ratio",
  "align-content align-items align-self justify-content justify-items justify-self flex-basis flex-direction flex-grow flex-shrink flex-wrap order gap row-gap column-gap",
  "grid-auto-columns grid-auto-flow grid-auto-rows grid-column-end grid-column-start grid-row-end grid-row-start grid-template-areas grid-template-columns grid-template-rows",
  "float clear overflow-x overflow-y overflow-wrap visibility opacity isolation mix-blend-mode",
  "margin-top margin-right margin-bottom margin-left padding-top padding-right padding-bottom padding-left",
  "background-attachment background-clip background-color background-image background-origin background-position-x background-position-y background-repeat background-size",
  "border-top-color border-top-style border-top-width border-right-color border-right-style border-right-width border-bottom-color border-bottom-style border-bottom-width border-left-color border-left-style border-left-width",
  "border-top-left-radius border-top-right-radius border-bottom-right-radius border-bottom-left-radius border-collapse border-spacing box-shadow",
  "color font-family font-feature-settings font-kerning font-optical-sizing font-size font-stretch font-style font-variant font-variation-settings font-weight",
  "letter-spacing line-break line-height tab-size text-align text-align-last text-decoration-color text-decoration-line text-decoration-style text-decoration-thickness text-indent text-overflow text-rendering text-shadow text-transform text-underline-offset vertical-align white-space word-break word-spacing writing-mode direction unicode-bidi hyphens",
  "filter backdrop-filter clip-path image-rendering object-fit object-position mask-image mask-position mask-repeat mask-size",
  "list-style-image list-style-position list-style-type table-layout",
  "transform transform-origin transform-style translate rotate scale",
  "color-interpolation fill fill-opacity fill-rule paint-order shape-rendering stop-color stop-opacity stroke stroke-dasharray stroke-dashoffset stroke-linecap stroke-linejoin stroke-miterlimit stroke-opacity stroke-width vector-effect",
] as const;

export const SCREENSHOT_STYLE_PROPERTIES = SCREENSHOT_STYLE_PROPERTY_GROUPS.flatMap((group) => group.split(" "));
