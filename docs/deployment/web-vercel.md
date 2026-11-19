# Web deploys (Vercel)

The web app (`www.agentclash.dev`) runs on Vercel. The GitHub workflow
`.github/workflows/web-deploy.yml` deploys it to production. The Vercel project
has no Git connection.

## Why GitHub Actions deploys the web app

Vercel counts each deploy toward the daily deploy limit of the team. This
includes a deploy that Vercel cancels at once, for example with
`git.deploymentEnabled` or an Ignored Build Step. With a connected repository,
each PR push made a canceled deploy. On 9 Nov 2026, those deploys used up the
limit, and Vercel refused all deploys for 24 hours.

Now only the workflow makes deploys. A PR push makes no deploy.

## When a deploy happens

- A push to `main` that changes a web input. The web inputs are `web/**` and
  the files outside `web/` that the build reads. The `paths` list of
  `web-deploy.yml` names them.
- A manual run: `gh workflow run web-deploy.yml --ref main`.

A PR gets no preview deploy. The `Frontend CI` workflow builds and tests it.

A deploy in progress always finishes. If more merges arrive during a deploy,
the workflow deploys only the newest commit after it.

## How the deploy works

The workflow runs `vercel deploy --prod` from the repository root. Vercel
builds the deploy with the settings of the project:

- The Root Directory is `web`, so Vercel uses `web/vercel.json`.
- The build can read files outside `web/`, for example
  `docs/api-server/openapi.yaml` and `cli/cmd`.
- The environment variables come from the Vercel project. The workflow sends
  none.

`git.deploymentEnabled: false` in `web/vercel.json` stops Git deploys if
somebody connects the repository again.

## Add a web input

If the web build reads a new file outside `web/`, add its path to the `paths`
list of `web-deploy.yml` and of `frontend.yml`. Keep the two lists the same.

## Roll back

1. Open the Vercel project, then **Deployments**.
2. Find the last good production deploy.
3. Select **Instant Rollback**.

The next merge that changes a web input deploys again. To stop that, fix the
problem first, or turn off the workflow in GitHub.

## One-time setup

1. Make the GitHub environment `vercel-production`. Let only the `main`
   branch deploy to it.
2. In Vercel, the team owner makes a token with the scope of the `agentclash`
   project only.
3. Copy the team ID and the project ID from the project settings.
4. Set the three secrets of the environment. `gh secret set` reads each value
   from the terminal, so the value does not go into the shell history:

   ```bash
   gh secret set VERCEL_TOKEN --env vercel-production
   gh secret set VERCEL_ORG_ID --env vercel-production
   gh secret set VERCEL_PROJECT_ID --env vercel-production
   ```

5. Run the workflow, and check the production site.
6. In Vercel, open the project **Settings**, then **Git**, and select
   **Disconnect**.

If a secret is missing, the workflow stops before the deploy and names the
secret.
