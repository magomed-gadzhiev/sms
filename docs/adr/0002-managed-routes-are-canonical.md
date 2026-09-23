# Managed (condition-based) routes are the canonical routing model

Routing vocabulary and all new routing features target the condition-based managed Route — operator/country/traffic-type/sender/regex conditions with schedules and weighted provider selection. The older pattern-based routes (prefix/exact/regex → single provider) are frozen as Legacy Route. The per-Client Routing Mode (`legacy|new|hybrid`) exists only as a transition mechanism while tenants migrate; the pattern-based engine must not be extended.

## Considered Options

- **Keep both generations equal**: a permanent vocabulary fork across two live routers; rejected.
- **Extend the pattern-based engine**: cannot express operator, traffic-type, or schedule conditions; rejected.
- **Managed routes canonical**: chosen — the richer model already backs reseller route sets and the staged pipeline.
