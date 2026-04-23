# main — Test Contract

## Functional Behavior
- Public landing page `Book a demo` CTAs should open `https://cal.com/atharva-kanherkar-epgztu/agentclash-demo`.
- Existing CTA styling, text, and placement should remain unchanged.
- Logged-out users should continue seeing the demo CTA in both landing-page CTA sections.
- The link should continue opening in a new tab so visitors land on the Cal.com booking UI.

## Unit Tests
- N/A — this change only updates a landing-page constant.

## Integration / Functional Tests
- Landing page renders both `Book a demo` buttons with the updated URL.
- Logged-in and logged-out branches still render the intended CTA sets.

## Smoke Tests
- `npm run lint` in `web/` passes for the modified file.

## E2E Tests
- N/A — not applicable for this copy/link update.

## Manual / cURL Tests
```bash
cd web
npm run lint -- src/app/landing.tsx
```

- Open the landing page locally.
- Verify both `Book a demo` buttons open the Cal.com booking page in a new tab.
