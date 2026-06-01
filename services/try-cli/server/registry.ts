import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { loadAllDemos, badgeSvg, type Demo } from "@try-cli/core";

const __dirname = dirname(fileURLToPath(import.meta.url));
const DEMOS_DIR = join(__dirname, "../../../try-cli/demos");

class DemoRegistry {
  private demos = new Map<string, Demo>();

  constructor() {
    this.reload();
  }

  reload() {
    this.demos.clear();
    for (const demo of loadAllDemos(DEMOS_DIR)) {
      this.demos.set(demo.slug, demo);
    }
  }

  get(slug: string): Demo | undefined {
    return this.demos.get(slug);
  }

  list(): Demo[] {
    return [...this.demos.values()].sort((a, b) => a.name.localeCompare(b.name));
  }
}

export const registry = new DemoRegistry();
