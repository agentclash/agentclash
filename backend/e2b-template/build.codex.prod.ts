import { Template, defaultBuildLogger } from 'e2b'
import { template } from './template'

async function main() {
  await Template.build(template, 'agentclash-codex-fullstack', {
    onBuildLogs: defaultBuildLogger(),
  });
}

main().catch(console.error);
