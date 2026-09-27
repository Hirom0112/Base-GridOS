# Integration data gaps

The simulated connectors in `services/control/internal/connectors/` provide typed substitutes. A `PENDING-LIVE` slot is not authorization to use operational data. Each production source below needs an approved agreement or API, scoped credentials, and access limited to the named site, member, device, or market role. The connector returns only data that source is authorized to disclose.

| Missing field from FULL_SPEC §7 | Connector supplying it | Production authorization requirement |
| --- | --- | --- |
| Real site/device IDs and the device-to-site relationship. | Battery telemetry and grid topology | Device owner or operator approval for the inventory and site binding. |
| Battery state of charge, power, usable energy, temperature, faults, state of health, cycle count, firmware, and last-seen time. | Battery telemetry | Authenticated gateway or device stream with site-scoped access. |
| Interval home load, solar, grid import/export, and critical-load behavior. | Household load | Household consent and authorized interval meter or gateway access. |
| Actual command acknowledgements and post-command telemetry. | Commands and battery telemetry | Authenticated command API and device telemetry permission for the same device. |
| Gateway/device protocols and command schemas. | Commands | Operator-approved protocol specification and command scope. |
| Identity, certificates, credentials, key rotation, and authorization model. | Commands and battery telemetry | Issuing authority and credential lifecycle agreement; no simulated credential is production authority. |
| Command acceptance, rejection, timeout, retry, expiry, and cancellation semantics. | Commands | Operator-approved command contract and authorization for each state transition. |
| Telemetry sequence, clock, quality, and reconnect behavior. | Battery telemetry | Authorized device stream with its timestamp and quality contract. |
| Approved home-to-transformer/feeder/substation mapping. | Grid topology | Utility approval for operational network mapping and site joins. |
| Real feeder limits, phase information, protection rules, and operating constraints. | Grid topology | Utility operational-data agreement and role-limited access. |
| Utility territory and program eligibility at site resolution. | Grid topology and member policy | Utility territory/program data rights and member consent for eligibility joins. |
| Member reserve preferences and consent history. | Member policy | Current member consent and audit access scoped to that member. |
| Resilience-plan selections, effective dates, and policy-change history. | Resilience and Travel Flex behavior | Member authorization and immutable selection history. |
| Scheduled/ended Travel Flex windows and early-return behavior. | Resilience and Travel Flex behavior | Member consent for each window and authorized return command. |
| Member reward response, opt-out, churn, complaint, and support outcomes. | Pricing and rewards; member policy | Member consent, support-system agreement, and purpose-limited access. |
| Consented away-period load baselines and anomaly-alert preferences. | Household load; resilience and Travel Flex behavior | Explicit household baseline and alert opt-in consent; public profiles do not qualify. |
| Critical-load definitions and backup-duration requirements. | Household load and member policy | Household-provided definition and consent for protected-load use. |
| Enrolled program, retail rate, bill history, incentives, and participation restrictions. | Member policy and pricing and rewards | Member enrollment consent plus utility/retailer billing and program data rights. |
| Effective-dated pricing catalogs, contract versions, and market-specific eligibility rules. | Pricing and rewards | Licensed catalog and executed commercial agreement with effective dates. |
| Actual fleet groupings, qualifications, bids, awards, and dispatch notices. | Market prices and regional load; settlement | Market participant authorization and approved operator feed. |
| Product-specific baseline and performance rules. | Settlement | Executed market product rules and licensed performance methodology. |
| Contracts, penalty curves, settlement statements, and realized revenue. | Settlement | Contract-party authorization and access to actual settlement statements. |
| Installation pipeline, inventory, crew capacity, service tickets, parts, and maintenance outcomes. | No §8 connector covers these operations; a deployment and maintenance connector is required. | Installer, operator, and service-provider agreements with site-scoped access. |
| Confirmed device reliability and degradation curves. | Battery telemetry for observed condition; a validated reliability source is still required. | Manufacturer or operator reliability data license and authorized device history. |
