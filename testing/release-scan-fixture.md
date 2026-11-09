# Release scan fixture repair — Test Contract

## Functional Behavior

Keep the recorded-conversation fake's evidence key and case-result key identical.
Use a short, clearly synthetic identifier. Preserve titles, messages, verdicts,
expectations, source selection and all application behavior. Do not change scanner
configuration or add exceptions.

## Unit Tests

No new unit test is needed for a two-literal test-fixture change. The existing
release source secret scan is the regression check: reproduce its failure before
the repair and require zero findings afterward with the pinned tool and flags.

## Integration / Functional Tests

Verify both fixture references use the same identifier. Run the existing mocked
browser journey for Improve with recorded replies, including its first result.

## Smoke Tests

Export tracked source and run the unchanged release secret scan on that export.
Require the protected release build to pass all checks and scans before publishing.

## E2E Tests

Run the recorded-replies browser journey with the repository Playwright config.
Use only its loopback fake; no external provider calls or credentials.

## Manual / cURL Tests

Review the exact diff. Require only the fixture references and this contract to
change. Keep operational records, test output and credentials outside Git.
