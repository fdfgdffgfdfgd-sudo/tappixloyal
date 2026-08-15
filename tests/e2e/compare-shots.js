// Compares two screenshot runs captured by visual-baseline.spec.ts.
// Usage: node tests/e2e/compare-shots.js [beforeDir] [afterDir]
const fs = require("node:fs");
const path = require("node:path");

const before = process.argv[2] ?? "shots-before";
const after = process.argv[3] ?? "shots-after";

if (!fs.existsSync(before) || !fs.existsSync(after)) {
  console.error(`Нет каталогов для сравнения: ${before}, ${after}`);
  process.exit(1);
}

let identical = 0;
const changed = [];
const missing = [];

for (const project of fs.readdirSync(before)) {
  for (const file of fs.readdirSync(path.join(before, project))) {
    const a = path.join(before, project, file);
    const b = path.join(after, project, file);
    if (!fs.existsSync(b)) {
      missing.push(`${project}/${file}`);
      continue;
    }
    if (fs.readFileSync(a).equals(fs.readFileSync(b))) identical++;
    else changed.push(`${project}/${file}`);
  }
}

console.log(`идентичных: ${identical}`);
console.log(`отличаются: ${changed.length}`);
changed.forEach(f => console.log(`    ${f}`));
if (missing.length) {
  console.log(`отсутствуют во втором прогоне: ${missing.length}`);
  missing.forEach(f => console.log(`    ${f}`));
}

process.exit(changed.length || missing.length ? 1 : 0);
