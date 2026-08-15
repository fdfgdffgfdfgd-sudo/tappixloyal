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

// These screens render timestamps and rows created by the run itself, so two
// captures of the same build differ. Verified by comparing a build against
// itself. They are reported, but they cannot prove or disprove a change.
const VOLATILE = ["_reports.png", "_risk-center.png", "_analytics.png"];

let identical = 0;
const changed = [];
const volatile = [];
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
    else if (VOLATILE.includes(file)) volatile.push(`${project}/${file}`);
    else changed.push(`${project}/${file}`);
  }
}

console.log(`идентичных: ${identical}`);
console.log(`отличаются: ${changed.length}`);
changed.forEach(f => console.log(`    ${f}`));
if (volatile.length) {
  console.log(`нестабильные по своей природе (не сигнал): ${volatile.length}`);
  volatile.forEach(f => console.log(`    ${f}`));
}
if (missing.length) {
  console.log(`отсутствуют во втором прогоне: ${missing.length}`);
  missing.forEach(f => console.log(`    ${f}`));
}

process.exit(changed.length || missing.length ? 1 : 0);
