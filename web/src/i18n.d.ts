// The globals of web/i18n.js, which stays a plain script loaded before app.js (web/i18n_test.go reads its one-entry-per-line
// form, and scripts/video.js runs it in a sandbox).
type Params = Record<string, unknown>;
declare const MESSAGES: Record<string, Record<string, string>>;
declare const LANGS: string[];
declare let currentLang: string;
declare function setLang(l: string): void;
declare function hasMsg(key: string): boolean;
declare function t(key: string, params?: Params): string;
declare function tn(key: string, n: number, params?: Params): string;
