# Exploration wire contract and rollout

Gateway MR !3264 pairs with semantic_model MR !31. Exploration is disabled by
default and runs only in EE/SaaS. Both direct and cached router constructors
reject exploration without execution feedback when a Router URL is configured.

## Wire contract

- `available_model_ids`: at most 4096 IDs, each at most 256 Unicode code points.
  Router source: `router_service/rank_input.py`, `RankRequest.available_model_ids`.
  Gateway omits the entire scope on overflow rather than truncating the Top2
  candidate set. Ordinary routing remains available without exploration scope.
- IDs match using lower-case, trimmed identity. Gateway deduplicates on that
  identity while retaining the original model ID on the wire. Unavailable and
  blank IDs are excluded.
- `routing_decision.version`: `simple-exploration-decision/1`, as declared in
  Router `router_service/exploration/contracts.py`.
- `target_model_id`: Gateway model ID, not benchmark candidate ID. Router
  `router_service/app.py` projects candidates through policy `requested_model`;
  `router_service/exploration/ledger.py` records the selected Top2 from that
  projected list. Gateway compares each actual attempt against that immutable
  target, including fallback attempts.

## Cache compatibility

The new decision is optional, additive metadata; existing cache formats remain
valid. A record created before this rollout contains no exploration decision.
Cache hits and lease renewals keep that original ranking and do not fabricate
exploration attribution. Attribution therefore becomes available progressively
on newly ranked turns. A lease renewal alone does not backfill metadata.

Do not flush or version-bump existing turn snapshots solely to populate the new
field: a same-turn cache miss intentionally cannot rerank, and would instead
force local fallback. Cross-cache preservation applies to decisions that were
present when the ranking was created, not to pre-rollout snapshots.

## Validation boundaries

Focused Gateway tests cover optional metadata validation, independent clones,
legacy snapshot reuse, constructor dependencies, candidate bounds and normalized
identity, and fallback attribution. This does not establish production load
capacity for maximum-sized catalogs, or a complete production learning loop.
Body linking, scoring, and daily sample-count publication remain out of scope.
