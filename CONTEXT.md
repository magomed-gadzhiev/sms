# SMS Platform

The ubiquitous language of the SMS/messaging platform: a hub that receives messages from customers over SMPP and HTTP, routes them through upstream providers, bills per segment, and reports delivery back.

## Tenancy & Actors

**Client**:
A customer account — the tenant that owns messages, campaigns, contacts, routes, and money.
_Avoid_: account (that is the wallet), tenant, "client" for SMPP connections

**Reseller**:
A Client that resells platform capacity to its own Sub Accounts under its brand. Russian UI says «агрегатор»; the portal section is "Network mode" — neither is a domain term.
_Avoid_: aggregator, network operator

**Sub Account**:
A Client owned by a Reseller; routing, pricing, and balance may be delegated to it.
_Avoid_: child client

**User**:
A portal login identity belonging to a Client — or to platform staff when it has no Client.
_Avoid_: client, account

**Role**:
An access role of a User: `admin`, `superadmin`, `client`, or `operator`. The `operator` Role means read-only staff (candidate for renaming to Support); it is never a mobile Operator.

**Account**:
The billing wallet of a Client: balance, credit limit, frozen funds. One per Client; never a login.
_Avoid_: login, profile, tenant

**Company**:
Legal-entity requisites (tax IDs, bank details) attached to a Client.
_Avoid_: client, tenant, organization

**Operator**:
A mobile network operator (MNO) identified by number prefixes; a destination attribute of a Message.
_Avoid_: the staff Role "operator" — say Support instead

## Messaging

**Message**:
One logical send request — the business unit a Client submits and tracks by External ID.
_Avoid_: SMS (that names one channel), PDU

**Segment**:
A physical part of a Message produced by splitting on length and encoding; the unit of transport counting and of billing.
_Avoid_: part; bare "segment" for audiences

**Saved Segment**:
A stored audience filter over contacts, reusable by campaigns.
_Avoid_: segment (reserved for SMS parts)

**PDU**:
An SMPP protocol wire unit; ephemeral, never a persisted domain concept.
_Avoid_: message, packet

**External ID**:
The Client-supplied identifier of a Message; the idempotency key, unique within the Client.
_Avoid_: message ID (ambiguous)

**Provider Message ID**:
The upstream provider's receipt identifier used to match Delivery Receipts to Messages.
_Avoid_: message ID, external ID

**Message Status**:
The Message lifecycle: `pending → queued → sent → delivered | failed | expired | rejected`, plus `scheduled` and `cancelled`. `unknown` is never a stored status — an UNKNOWN receipt leaves the Message in `sent` until the DLR timeout expires it.

**Delivery Receipt (DLR)**:
A terminal delivery confirmation from the provider about a Message.
_Avoid_: webhook (that is the client-facing callback), status event

**Channel**:
The transport carrying a send to the subscriber: `sms`, `max`, `viber`, `hlr`, `flash_call`, `reverse_call`, `messenger`. One orthogonal axis; route types are not channels.
_Avoid_: route type

**Traffic Type**:
The commercial category of traffic: `transactional`, `marketing`, or `service` (`authorization`/`extensible` are legacy values).
_Avoid_: message type

## Routing & Providers

**Route**:
A managed rule that matches messages by operator, country, traffic type, sender category, or regex — with schedules — and points to one or more Providers. The canonical meaning.
_Avoid_: pattern route

**Legacy Route**:
A prefix/exact/regex pattern rule of the older routing generation; frozen, receives no new features.

**Client Route**:
A Route owned by a specific Client rather than the platform.

**Routing Mode**:
The per-Client switch deciding which route generations apply: `legacy`, `new`, or `hybrid`.

**Routing Resolution**:
The domain rule of routing precedence: the Client's own routes first, then its Reseller's shared route sets, then the platform default; first match wins.

**Priority**:
Route preference order: a higher number is preferred. It is never a price rank.

**Provider**:
An upstream SMSC endpoint the platform submits messages through.
_Avoid_: gateway, channel

**Gateway**:
A customer-facing ingress (HTTP API or SMPP). The egress side to providers is never called a gateway.

**SMPP Session**:
A customer's bound SMPP connection to the platform.
_Avoid_: "client" (reserved for the tenant)

**Sender Name**:
The registered source address a message appears from (alpha name).

**Sender Registration**:
The registration of a Sender Name at an Operator; paid or free, with its own lifecycle.

## Money

**Price**:
What a Client pays per Segment.
_Avoid_: rate, tariff, cost

**Cost**:
What the platform pays a Provider per Segment.
_Avoid_: price (client-side only)

**Price Rule**:
A rule of the unified pricing model: owned by platform, reseller, or sub account, and resolving Price by country, operator, sender category, traffic type, and date.
_Avoid_: pricing rule (legacy), tariff plan

**Tariff**:
Legacy umbrella for the older tariff/aggregator/reseller pricing families, superseded by Price Rules; do not use in new designs.

**Charge**:
An idempotent deduction of Price × segments from an Account.
_Avoid_: credit (a transaction direction), fee

**Charger**:
The billing-owned port for per-Message charges: idempotent on the commit guard, returning one typed outcome — Committed, AlreadyCommitted, Rejected (with reason), or Transient.
_Avoid_: charge service (that is a process, not the port)

**Margin**:
Price minus Cost for the same Segment.

**Quota**:
A Reseller's segment allowance for a period, with overage handling.

## Campaigns & Audiences

**Campaign**:
A bulk broadcast to an audience, materialized into individual Messages per Recipient.

**Recipient**:
One campaign row binding a contact to its Message and its delivery outcome.

**Contact List**:
A Client-owned list of contacts with custom attributes.
_Avoid_: group

**Contact**:
A phone number with attributes and tags, unique within its Contact List.

**Contact Import**:
A CSV upload job turning rows into Contacts.

**Opt-Out List**:
The suppression list of phone numbers that must never receive messages.
_Avoid_: blacklist

**Template**:
A message body with variables, subject to an approval workflow.
_Avoid_: operator template

**Operator Template**:
Content registered with an Operator for compliance; a separate concept from Template.

**Webhook**:
An HTTPS callback to a Client on message events: `delivered`, `failed`, `expired`, `rejected`.

## Cascades

**Delivery**:
One multi-channel send governed by a Delivery Strategy.
_Avoid_: message (that is the single-channel instance)

**Delivery Strategy**:
The sequential or parallel set of channel steps, with timeouts, that a Delivery follows.

**Delivery Attempt**:
The execution of one strategy step; an attempt over an SMS-family channel spawns a Message.
