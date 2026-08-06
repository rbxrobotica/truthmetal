# TruthMetal golden suite contract v1

This contract records TruthMetal acceptance of a versioned offline evaluation
suite. It does not execute evaluations, route models, gate traffic, publish a
signing key, or authorize rollout.

## Decision context

ADR-0023 is the accepted authority for scoped RAG packages. It assigns golden
case acceptance to TruthMetal, asynchronous measurement to Verentir, and runtime
control to Thalamus. ADR-0020 is still proposed and is cited only as additional
consumer-boundary context for Verentir.

## Files

Each suite directory contains exactly:

- `manifest.json`: acceptance authority, immutable source provenance, artifact
  hashes, exclusions, signing status, and rollout status.
- `quality-gates.json`: the accepted threshold contract consumed by
  `verentir-rag-quality/1`.
- `cases.json`: accepted golden cases consumed by the Verentir shadow evaluator.

The checked-in validator rejects unknown fields, symbolic links, hash drift,
non-TruthMetal ownership, non-shadow execution, relaxed human approval, missing
evidence policy, duplicate cases, blocked evidence, and a signing key published
while its trust anchor remains pending.

## Acceptance semantics

An `accepted` file on a feature branch is a proposal. Acceptance becomes
authoritative only when a separate human review merges it into protected
`truthmetal/main`. Consumers must resolve the suite from `refs/heads/main`
or a commit reachable from that ref. Later changes require another reviewed
commit.

The public-assistant v1 suite accepts 30 of the 38 package candidates. Eight cases
that reference the public Strategos document are excluded because the claims
register still marks the Strategos and product-ownership claim families as
missing evidence. Their IDs and reasons remain explicit in the manifest.

The accepted Robson cases contain no prohibited return, precision, urgency, or
autotrader promise. They encode factual limitations and financial refusals. The
pending legal disclaimer remains a separate rollout requirement.

## Compatibility

This is an additive offline contract. It changes no HTTP, gRPC, database, auth,
or runtime interface. Verentir is the only intended consumer and can read the
accepted `quality-gates.json` and `cases.json` using its existing v1 adapters.

The v1 JSON shape is exact. Removing, renaming, or adding a field, changing field
semantics, or changing accepted case identity requires a new versioned suite
directory and an explicit consumer compatibility review.

## Remaining blockers

The suite deliberately publishes no Ed25519 trust anchor. Therefore signed
quality reports remain ineligible. The quality contract also preserves these
requirements:

- Verentir must produce three consecutive passing shadow reports.
- Every answer must cite only current authorized evidence.
- Strategos and product-ownership claims need permitted evidence.
- The Robson disclaimer needs legal approval and compliance review.
- Public rollout requires a separate human approval.

No merge of this suite deploys code, changes production data, or authorizes a
shadow campaign.

Rollback is a revert of the merge commit. There is no migration, external write,
or generated operational state to undo.
