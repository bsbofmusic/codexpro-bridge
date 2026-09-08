# CodexPro Bridge — Archived Public Snapshot

This repository is retired and is no longer an installable or deployable source for CodexPro Bridge.

The historical 0.1.x tree exposed an obsolete eight-tool ChatGPT/MCP surface and included Plugin application manifests. Keeping those artifacts live created a second registration source that could cause stale ChatGPT application schemas to reappear after the production Bridge had moved on.

As of 2026-09-08:

- this repository contains no active MCP server package;
- this repository contains no ChatGPT Plugin/App manifest;
- this repository must not be used to create or refresh a CodexPro Bridge connection;
- historical source remains available only through Git history and the immutable `v0.1.0` / `v0.1.1` tags.

The production Bridge has a separately governed runtime and release lifecycle. Its current tool schema must be obtained from the live MCP endpoint itself with MCP `initialize` + `tools/list`, never from this archived repository.

## Why this was archived

The old public snapshot was a valid historical release, but it became unsafe as an operational source once the production Bridge evolved independently. The duplicate package name, Plugin display name, application manifest, and fixed eight-tool contract made it possible to re-register an obsolete ChatGPT application while the server itself was already on a newer schema.

Archiving the installable surface enforces one current source of truth and prevents accidental rollback-by-registration.

## Historical access

Use Git history or the signed/annotated release tags for archaeology only. Do not restore the old Plugin manifests or eight-tool contract into a current client.
