#!/usr/bin/env node
// Держит оформление на одном источнике значений.
//
// До появления tokens.css :root определялся в пяти файлах, и фирменный цвет
// зависел от порядка импортов в layout.tsx — получилось пять разных акцентов,
// ни один из которых не совпадал с MASTER.md. Проверки ниже описаны шире, чем
// та правка: они ловят не конкретные пять значений, а само возвращение второго
// источника, любой из вытесненных акцентов и нарушение контраста.

const fs = require("node:fs");
const path = require("node:path");

const ROOT = path.resolve(__dirname, "../..");
const APP = path.join(ROOT, "apps/business/app");
const TOKENS = "tokens.css";

// Акценты предыдущих поколений. Любой из них в коде означает, что цвет снова
// живёт мимо токена.
const SUPERSEDED = ["#6952e8", "#5b4bce", "#6558d9", "#5b55e7", "#2563eb", "#5540cf", "#5548c8"];

// Токены, без которых MASTER.md не реализован.
const REQUIRED = [
  "--app-bg", "--surface", "--surface-subtle", "--text", "--text-muted", "--border",
  "--accent", "--accent-hover", "--accent-soft", "--success", "--success-soft",
  "--warning", "--warning-soft", "--danger", "--danger-soft", "--focus",
];

// Пары, для которых MASTER.md требует AA 4.5:1.
const CONTRAST_PAIRS = [
  ["--text", "--surface"], ["--text", "--app-bg"], ["--text", "--surface-subtle"],
  ["--text-muted", "--surface"], ["--text-muted", "--app-bg"], ["--text-muted", "--surface-subtle"],
  ["--accent", "--surface"], ["--success", "--surface"], ["--warning", "--surface"], ["--danger", "--surface"],
];
const AA = 4.5;

const problems = [];

function walk(dir, exts) {
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) return entry.name === "node_modules" || entry.name === ".next" ? [] : walk(full, exts);
    return exts.some((e) => entry.name.endsWith(e)) ? [full] : [];
  });
}

function relative(file) {
  return path.relative(ROOT, file).split(path.sep).join("/");
}

// 1. Единственный :root.
for (const file of walk(APP, [".css"])) {
  if (path.basename(file) === TOKENS) continue;
  if (/:root\s*\{/.test(fs.readFileSync(file, "utf8"))) {
    problems.push(`${relative(file)}: определяет :root. Значения живут только в app/${TOKENS}.`);
  }
}

// 2. Вытесненные акценты.
const sources = [...walk(APP, [".css", ".tsx"]), ...walk(path.join(ROOT, "apps/business/components"), [".tsx"])];
for (const file of sources) {
  if (path.basename(file) === TOKENS) continue;
  const text = fs.readFileSync(file, "utf8").toLowerCase();
  for (const hex of SUPERSEDED) {
    if (text.includes(hex)) {
      problems.push(`${relative(file)}: содержит ${hex} — акцент прошлого поколения. Используйте var(--accent).`);
    }
  }
}

// 3. Палитра MASTER.md реализована, и значения читаются.
const tokensCss = fs.readFileSync(path.join(APP, TOKENS), "utf8");
const declared = new Map();
for (const [, name, value] of tokensCss.matchAll(/(--[a-z0-9-]+)\s*:\s*([^;}]+)/gi)) {
  declared.set(name.trim(), value.trim());
}
for (const token of REQUIRED) {
  if (!declared.has(token)) problems.push(`app/${TOKENS}: нет токена ${token} из MASTER.md.`);
}

// 4. Контраст — MASTER.md требует AA 4.5:1 для обычного текста.
function channel(v) {
  const c = v / 255;
  return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
}
function luminance(hex) {
  const h = hex.replace("#", "");
  const full = h.length === 3 ? [...h].map((c) => c + c).join("") : h;
  const [r, g, b] = [0, 2, 4].map((i) => channel(parseInt(full.slice(i, i + 2), 16)));
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}
function ratio(a, b) {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}
function hex(value) {
  if (!value) return null;
  let v = value.trim();
  for (let depth = 0; depth < 8; depth += 1) {
    const ref = /^var\(\s*(--[a-z0-9-]+)\s*\)$/i.exec(v);
    if (!ref) break;
    v = (declared.get(ref[1]) || "").trim();
  }
  v = v.replace(/\s*!important\s*$/i, "").trim();
  return /^#[0-9a-f]{3}$|^#[0-9a-f]{6}$/i.test(v) ? v : null;
}

for (const [fg, bg] of CONTRAST_PAIRS) {
  const a = hex(`var(${fg})`);
  const b = hex(`var(${bg})`);
  if (!a || !b) continue;
  const value = ratio(a, b);
  if (value < AA) {
    problems.push(`контраст ${fg} на ${bg} = ${value.toFixed(2)}:1, требуется ${AA}:1 (MASTER.md, Accessibility).`);
  }
}

// Контраст реально отрисованных сочетаний здесь не проверяется намеренно.
// Статически не видно ни каскада, ни того, содержит ли элемент текст или иконку,
// поэтому такая проверка даёт десятки ложных срабатываний на перебитых правилах.
// Это делает axe в tests/e2e/accessibility.spec.ts — на живых страницах, с
// настоящим каскадом и правильным порогом для графики. Именно он поймал
// регрессию, которую пары токенов выше пропустили.

if (problems.length > 0) {
  console.error("Проверка токенов оформления не пройдена:\n");
  for (const problem of problems) console.error("  - " + problem);
  console.error(`\nВсего замечаний: ${problems.length}. Канон: design-system/tappix/MASTER.md`);
  process.exit(1);
}
console.log(`Токены оформления в порядке: один :root, ${REQUIRED.length} семантических токенов, ${CONTRAST_PAIRS.length} пар проходят AA.`);
