# Segment is the billing unit

Clients are billed per Segment — the physical SMS part after length/encoding splitting — not per Message, because every real cost driver (multipart messages, encoding, provider accounting, margin math) is per segment, and the unified pricing model (price per segment everywhere) already assumes it. The legacy per-message pricing (`ChargeMessage`, the old `pricing_rules`) is deprecated and must not be extended.

## Considered Options

- **Per Message** (legacy): simpler to reason about, but undercharges multipart sends and diverges from provider-side accounting; rejected.
- **Per Segment**: chosen — matches transport reality and the direction of the unified pricing model.
