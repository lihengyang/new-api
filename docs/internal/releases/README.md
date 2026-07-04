# Internal Release Records

This directory stores internal release notes, rollout evidence, and customer
documentation closure records. These files are internal-only and may contain
sanitized operational evidence that must not be copied into customer docs.

## Active Customer Onboarding Guide

- Current guide:
  `Light_Speed_Future_API_Integration_Guide_v2.2.0_Seed_Audio_Production_Edition.docx`
- v2.2.0 supersedes v2.1.4 for active onboarding unless a tenant-specific note
  says otherwise.
- Final DOCX/PDF visual QA authority: Microsoft Word / Office / Henry manual
  review.
- LibreOffice render output is only a quick automated preview and structure
  smoke.

## Seed Audio P0

- Record: `2026-06-29-seed-audio-v1-p0.md`
- Final status:
  `PRODUCTION_RELEASED / CUSTOMER_DOC_READY / TEXT_AUDIO_IMAGE_VALIDATED`
- Customer-facing capabilities:
  `POST /v1/audio/speech`, `text_only`, `audio_url`, `image_url`,
  `metadata.client_request_id`, and `GET /v1/billing/balance`.
- Reference URL caveat: public HTTPS and provider-accessible are required;
  browser-accessible does not guarantee provider-side access or processing.

## Seed Audio Parser Hotfix

- Record: `2026-07-04-seed-audio-parser-hotfix-production.md`
- Final status: `PRODUCTION_DEPLOY_PASSED`
- Production image:
  `new-api:seed-audio-parser-hotfix-rc1-dcda4e55`
- Source / OCI revision:
  `dcda4e551e50051ab536173f342eb21f0ca60e8d`
- Scope: internal response-adapter hotfix for official top-level Seed Audio
  success schema parsing. Customer documentation was not changed.
- Internal smoke used the Henry/LSF `henrytest` testing account/model entry,
  not a customer token, customer alias, customer group, or customer balance.

## Documentation Guardrails

- Do not publish customer files with `_FIXED`, `_DRAFT`, `_REJECTED`, or wrong
  version filenames.
- Do not expose ProjectName, upstream model names, internal channel/group/project
  configuration, quota formulas, group ratios, test balances, raw prompts, raw
  reference URLs, temporary output URLs, or production smoke details in customer
  docs.
- Do not let docs-only commits recursively change production candidate image
  tags, source revisions, or deployed artifact traceability.
