import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { auditDocumentLinks } from "./document-links.mjs";

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, "../..");
const output = path.resolve(root, process.argv[2] || ".e2e/review-followup-20261010/document-links-final.json");
const report = auditDocumentLinks(root);
await mkdir(path.dirname(output), { recursive: true });
await writeFile(output, JSON.stringify(report, null, 2) + "\n");
console.log(`Audited ${report.stats.files} files and ${report.stats.links + report.stats.images} Markdown references: ${report.stats.broken} broken.`);
if (report.errors.length) process.exitCode = 1;
