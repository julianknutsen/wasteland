# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in Wasteland, please report it responsibly:

1. **Do not** open a public issue for security vulnerabilities
2. Email the maintainers directly with details
3. Include steps to reproduce the vulnerability
4. Allow reasonable time for a fix before public disclosure

## Scope

Wasteland is experimental software focused on DoltHub federation. Security considerations include:

- **Dolt operations**: Wasteland executes dolt CLI commands as the running user
- **DoltHub federation**: Wasteland communicates with DoltHub APIs using user tokens
- **Shell execution**: SQL scripts are executed via dolt subprocess calls
- **Config data**: Configuration and federation state stored in XDG directories

## Best Practices

When using Wasteland:

- Keep DOLTHUB_TOKEN secure and never commit it to version control
- Review wanted board content before acting on it
- Use appropriate DoltHub permissions for your organization
- Monitor federation activity via `wl browse` and `wl sync`

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

## Updates

Security updates will be released as patch versions when applicable.
