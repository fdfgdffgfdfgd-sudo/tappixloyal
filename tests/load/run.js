// Runs the k6 smoke against a stack that is already up.
//
// This exists because the npm script used "$PWD", which cmd.exe does not
// expand, so `npm run test:load` failed on Windows with "invalid characters
// for a local volume name". Node resolves the path the same way everywhere.
const { spawnSync } = require("node:child_process");
const path = require("node:path");

const scripts = path.resolve(__dirname);
const args = [
  "run", "--rm",
  "--add-host=host.docker.internal:host-gateway",
  "-e", `VUS=${process.env.VUS ?? "2"}`,
  "-e", `DURATION=${process.env.DURATION ?? "20s"}`,
];
if (process.env.BASE_URL) args.push("-e", `BASE_URL=${process.env.BASE_URL}`);
args.push("-v", `${scripts}:/scripts:ro`, "grafana/k6:0.57.0", "run", "/scripts/smoke.js");

const result = spawnSync("docker", args, { stdio: "inherit" });
process.exit(result.status ?? 1);
