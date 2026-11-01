# Registration reward eligibility (SaaS)

Issue: [starhub-server#1607](https://git-devops.opencsg.com/product/starhub/starhub-server/-/work_items/1607)

## Decision

Registration succeeds regardless of email or phone classification. Self-registration credit (activity 1005) and invitee registration credit (activity 1003) are delayed until both contact fields are present and neither matches an enabled risk rule. The inviter's later award also rechecks the invitee's current contacts before payout. CE and EE do not execute this feature.

## Rules

- Store disposable email domains and virtual-operator phone prefixes in SaaS database rules. Operators may add or disable rows directly in the database; no administration API is required.
- Normalize an email domain to lowercase and match that domain or a subdomain on a label boundary. Normalize the phone area to an international calling code and compare the national number with prefixes for that area only. A region without rules is not classified as virtual.
- Seed email domains from the MIT-licensed [Castle list](https://github.com/castle/disposable-email-domains), source blob SHA `8c8678c544311922ccb9c13a4cbf1d3f1d3e4111`. The license notice is in `CASTLE_LICENSE.txt`. Seed +86 prefixes `162`, `165`, `167`, `170`, `171`, and every three-digit prefix `140` through `149` inclusive. The latter intentionally delays awards for every such number, including possible ordinary-carrier users.
- A rule lookup failure delays the award and is logged as an operational error. A match or incomplete contact data is a normal pending outcome, not a registration error.
- SaaS `GET /user/{username}` returns `email_risk_status` and `phone_risk_status` for full profiles only. The fields describe contact risk, not reward status. Each independently reports `normal`, `suspicious`, `missing`, or `unknown`; an unavailable rule query or a phone without its calling code is `unknown`. Anonymous and other basic profiles omit both fields. CE and EE omit them.

Operators add email rules to `risk_email_domains (domain, enabled)` and phone rules to `risk_phone_prefixes (phone_area, prefix, enabled)`. Phone calling codes include `+`. Domains are unique; phone prefixes are unique within a calling code. Disable either rule by setting `enabled = FALSE`.

Database access in `builder/store/database` only loads enabled rules. The shared `builder/store/cache` layer owns in-memory snapshots and refresh coordination. SaaS components own contact normalization, matching, and eligibility decisions; profile and reward checks share the same checker and process-local snapshots for the active database. Email domains and calling-code/prefix pairs are loaded into separate immutable hash sets. Warm lookups use memory only. Each snapshot expires after one minute and is refreshed on the next request, with concurrent refreshes merged into one database load. Database edits therefore take effect on the next lookup after the snapshot expires. An expired snapshot is not reused after a refresh failure: that contact returns `unknown` and awards remain pending. Errors are cached for five seconds to limit retries during a database outage. Each refresh has a five-second timeout; cancelling one request does not cancel a refresh used by other requests.

Deploy the migration before starting the new SaaS user service. The migration checks existing activity 1003/1005 awards for duplicate `(user_uuid, activity_id)` pairs before adding a unique index; reconcile any such records before retrying a failed migration. If the rules table is unavailable, registration continues while awards remain pending.

Rollback removes only the registration-award unique index and preserves both risk-rule tables, including operator additions and disabled seeds. Up/Down may be repeated; reapplying Up inserts missing seeds without overwriting existing rules. Dropping the retained tables requires a separate deliberate data-cleanup operation.

## Retry and accounting

Re-evaluate self-registration and invitee registration awards after account creation and each actual email or phone change, including changes from one nonempty value to another. Those delayed registration awards are not scanned automatically when an operator changes the rules; the next contact change triggers another attempt. Accounting must commit an award at most once per user and activity even if two entry points try concurrently.

Before each monthly inviter payout, re-read the invitee and apply the same email and phone rules. An absent or soft-deleted invitee is logged and skipped without a payout or error; the activity remains pending so restoring the user can permit a later monthly retry. Other user lookup failures still fail the attempt. Missing or suspicious contacts leave the inviter activity pending; lookup failures fail that payout attempt without advancing its status. Monthly runs include pending activities from all completed months, so a corrected contact can be paid on a later run. The batch uses an ID cursor so activities settled during one run do not cause later pending activities to be skipped.

## Acceptance

- Registration succeeds for listed addresses and numbers, with no registration credit issued.
- Updating the offending contact to an unlisted value issues the applicable award once; repeating the update or triggering both paths concurrently never duplicates it.
- Email case and subdomains, +86 prefixes, area isolation, missing contacts, rule query errors, migration up/down, and existing non-invited/invited paths are covered by tests.
- Full user profiles expose independent email and phone risk statuses; basic profiles never expose them, and rule lookup failures display `unknown` without failing the profile request.
- Monthly inviter payouts recheck current invitee contacts before any status or accounting change. Suspicious, missing, and lookup-failure cases do not pay; older pending activities remain eligible for later monthly runs.
- Repeated warm lookups perform no SQL; expiry applies rule additions and deactivations. Concurrent cold requests share one load, cancelled callers do not interrupt other callers, and failed refreshes never fall back to stale eligibility decisions.
