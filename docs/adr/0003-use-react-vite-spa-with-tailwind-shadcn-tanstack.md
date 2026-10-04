# ADR-0003: Use a React and TypeScript single-page app with Tailwind CSS, shadcn/ui, TanStack and axios

- Status: Accepted
- Date: 2026-10-01
- Task: none
- Deciders: Karthik Reddy
- Area: frontend
- Reversibility: awkward: every screen is built on this stack; swapping the UI kit or data layer later means rewriting components

## Context

- From the request: "react and tailwindcss and shadcn and typescript and axios and tanstack for the frontend".
- Catalift is an app used by signed-in sellers and reviewers (PRD section 4); no page needs search engine indexing, so server rendering adds nothing.
- The core screen is a review grid with editing, filtering and bulk selection (US-00-007, US-00-009), and long-running jobs whose progress the UI polls (US-00-003).
- The app is served as static files from the VM's reverse proxy (ADR-0002), which a single-page app fits.

## What else was considered

| Option | Why not | Would suit |
| --- | --- | --- |
| React + Vite + TypeScript, Tailwind, shadcn/ui, TanStack Query and Table, axios (chosen) | shadcn copies component source into the repo, so its updates are manual; axios duplicates what `fetch` already does | an app behind sign-in with data grids and server state |
| Next.js | server rendering and SEO are not needed here, and it needs a Node server on the VM | marketing pages or public, indexable content |
| fetch instead of axios | a few more lines for interceptors and error handling | a team that wants one less dependency |

## Decision

We will build the frontend as a React single-page app with Vite and TypeScript, styled with Tailwind CSS and shadcn/ui, with TanStack Query for server state and polling job progress, TanStack Table for the review grid, and axios as the HTTP client behind one typed API module, because it is the standard choice for a signed-in app, TanStack Table fits the grid stories directly, and it deploys as static files on the existing VM.

## Consequences

- Vite is assumed as the build tool, since the request named React but no framework.
- TanStack Router is not chosen here; React Router or TanStack Router is a later, cheap choice.
- All HTTP calls go through one axios instance (base URL, error mapping), and TanStack Query is the only place components fetch data.
- Revisit if the product adds public, indexable pages (then Next.js), or if the grid outgrows TanStack Table at thousands of rows (then add row virtualisation first).

## Commits us to

React, TypeScript, Vite, Tailwind CSS, shadcn/ui (Radix UI underneath), TanStack Query, TanStack Table, axios
