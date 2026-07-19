# Security Policy

## Reporting a Vulnerability

Please do not report security vulnerabilities through public GitHub issues.

Send security reports to support@deeix.com with:

- a clear description of the issue
- affected version or commit
- reproduction steps or proof of concept
- impact assessment
- any relevant logs, screenshots, or request samples

We will acknowledge valid reports as soon as possible and coordinate remediation before public disclosure.

## Scope

Security reports are in scope when they affect DEEIX Chat source code, default deployment guidance, authentication, authorization, data isolation, secret handling, file processing, model provider routing, MCP/tool execution, billing, or administrative APIs.

Out of scope:

- reports requiring physical access to a user's device
- social engineering
- denial-of-service reports without a practical exploit path
- vulnerabilities in third-party services that are not caused by this project

## Supported Versions

The project is under active development. Security fixes are applied to the main branch first and released with the next public version unless a dedicated patch release is needed.

## Deployment Security

Production deployments must use production configuration, strong secrets, HTTPS, restricted CORS, and a production database. Do not deploy the repository's local development `config.yaml`.
