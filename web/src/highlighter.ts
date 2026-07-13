import { createHighlighterCore, type LanguageInput } from "@shikijs/core";
import { createJavaScriptRegexEngine } from "@shikijs/engine-javascript";
import githubDark from "@shikijs/themes/github-dark";

type LanguageLoader = () => Promise<{ default: LanguageInput }>;

const languages: Record<string, LanguageLoader> = {
  c: () => import("@shikijs/langs/c"),
  cpp: () => import("@shikijs/langs/cpp"),
  css: () => import("@shikijs/langs/css"),
  csv: () => import("@shikijs/langs/csv"),
  dockerfile: () => import("@shikijs/langs/dockerfile"),
  go: () => import("@shikijs/langs/go"),
  html: () => import("@shikijs/langs/html"),
  java: () => import("@shikijs/langs/java"),
  javascript: () => import("@shikijs/langs/javascript"),
  json: () => import("@shikijs/langs/json"),
  jsx: () => import("@shikijs/langs/jsx"),
  kotlin: () => import("@shikijs/langs/kotlin"),
  lua: () => import("@shikijs/langs/lua"),
  make: () => import("@shikijs/langs/make"),
  markdown: () => import("@shikijs/langs/markdown"),
  php: () => import("@shikijs/langs/php"),
  python: () => import("@shikijs/langs/python"),
  ruby: () => import("@shikijs/langs/ruby"),
  rust: () => import("@shikijs/langs/rust"),
  scss: () => import("@shikijs/langs/scss"),
  shellscript: () => import("@shikijs/langs/shellscript"),
  sql: () => import("@shikijs/langs/sql"),
  toml: () => import("@shikijs/langs/toml"),
  tsx: () => import("@shikijs/langs/tsx"),
  typescript: () => import("@shikijs/langs/typescript"),
  xml: () => import("@shikijs/langs/xml"),
  yaml: () => import("@shikijs/langs/yaml"),
};

const languageLabels: Record<string, string> = {
  cpp: "C++",
  csv: "CSV",
  html: "HTML",
  javascript: "JavaScript",
  json: "JSON",
  jsx: "JSX",
  php: "PHP",
  scss: "SCSS",
  shellscript: "Shell",
  sql: "SQL",
  toml: "TOML",
  tsx: "TSX",
  typescript: "TypeScript",
  xml: "XML",
  yaml: "YAML",
};

export const highlightLanguageOptions = [
  { value: "text", label: "Plain text" },
  ...Object.keys(languages).map((value) => ({
    value,
    label:
      languageLabels[value] ?? value.charAt(0).toUpperCase() + value.slice(1),
  })),
];

const highlighter = createHighlighterCore({
  themes: [githubDark],
  langs: [],
  engine: createJavaScriptRegexEngine(),
});

export async function highlightCode(source: string, language: string) {
  const instance = await highlighter;
  const loader = languages[language];
  if (!loader)
    return instance.codeToHtml(source, { lang: "text", theme: "github-dark" });
  if (!instance.getLoadedLanguages().includes(language)) {
    await instance.loadLanguage((await loader()).default);
  }
  return instance.codeToHtml(source, { lang: language, theme: "github-dark" });
}
