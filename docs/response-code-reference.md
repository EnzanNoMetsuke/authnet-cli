# Curated Response-Code Reference

`authnet response-code explain` uses a checked-in Curated response-code reference.
It must not fetch documentation at runtime.

## Sources

Review these official Authorize.Net sources before changing the local reference:

- Authorize.Net response-code tool: <https://developer.authorize.net/api/reference/responseCodes.html>
- Authorize.Net API error and response codes: <https://developer.authorize.net/api/reference/features/errorandresponsecodes.html>
- Authorize.Net API transaction response fields: <https://developer.authorize.net/api/reference/index.html>

## Maintenance Path

When updating the reference:

1. Review the official sources above.
2. Update `internal/cli/response_codes.go`.
3. Update the reference version and review date.
4. Keep Source meaning separate from project-authored Recommended next steps.
5. Add or update tests for every changed Code family or ambiguous code.
6. Run `GOTMPDIR="$PWD/.cache/go-tmp" make verify`.
