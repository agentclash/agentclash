import { Template, defaultBuildLogger } from 'e2b'
import { template } from './template'

async function main() {
  await Template.build(template, 'agentclash-claude-fullstack-dev', {
    onBuildLogs: defaultBuildLogger(),
  });
}

main().catch((err) => {
  console.error(err);
  process.exitCode = 1;
});
