const siteTitle = "sql garden"
const siteDescription = "the garden of sql design and solace"
const siteImage = "https://s3.amazonaws.com/pix.iemoji.com/images/emoji/apple/ios-12/256/seedling.png"
const siteImageAlt = "seedling emoji"
const siteName = siteTitle

// See https://observablehq.com/framework/config for documentation.
export default {
  // The app’s title; used in the sidebar and webpage titles.
  title: "sql garden",

  // The pages and sections in the sidebar. If you don’t specify this option,
  // all pages will be listed in alphabetical order. Listing pages explicitly
  // lets you organize them into sections and have unlisted pages.
  // pages: [
  //   {
  //     name: "Examples",
  //     pages: [
  //       {name: "Dashboard", path: "/example-dashboard"},
  //       {name: "Report", path: "/example-report"}
  //     ]
  //   }
  // ],

  // Content to add to the head of the page, e.g. for a favicon:
  head: `
  <!-- Basic site info-->
  <meta name="title" data-hid="title" content="${siteTitle}">
  <meta name="description" data-hid="description" content="${siteDescription}">
  
  <!-- Open Graph (Facebook/Linkedin) tags -->
  <!-- Testing tool: https://developers.facebook.com/tools/debug/ -->
  <meta property="og:site_name" content="${siteName}">
  <meta property="og:locale" content="en_US">
  <meta property="og:url" content="https://dofga.ope.cool">
  <meta property="og:type" content="website">
  <meta property="og:title" content="${siteTitle}">
  <meta property="og:description" content="${siteDescription}">
  <meta property="og:image" content="${siteImage}">
  
  <!-- Twitter tags -->
  <!-- Testing tool: https://cards-dev.twitter.com/validator -->
  <meta name="twitter:site" content="https://dofga.ope.cool">
  <meta name="twitter:card" content="summary">
  <meta name="twitter:title" content="${siteTitle}">
  <meta name="twitter:description" content="${siteDescription}">
  <meta name="twitter:image" content="${siteImage}">
  <meta name="twitter:image:alt" content="${siteImageAlt}">
  <title>DOFGA DATA</title>
  
  <link rel="icon" href="data:image/svg+xml,<svg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 100 100%22><text y=%22.9em%22 font-size=%2290%22>🌱</text></svg>">
    `,

  // The path to the source root.
  root: "src",

  // Some additional configuration options and their defaults:
  // theme: "default", // try "light", "dark", "slate", etc.
  // header: "", // what to show in the header (HTML)
  // footer: "Built with Observable.", // what to show in the footer (HTML)
  // sidebar: true, // whether to show the sidebar
  // toc: true, // whether to show the table of contents
  // pager: true, // whether to show previous & next links in the footer
  // output: "dist", // path to the output root for build
  // search: true, // activate search
  // linkify: true, // convert URLs in Markdown to links
  // typographer: false, // smart quotes and other typographic improvements
  // preserveExtension: false, // drop .html from URLs
  // preserveIndex: false, // drop /index from URLs
};
