// Mirrors the Go domain types across internal/user, internal/provider,
// internal/security and internal/audit. Kept as one file since the
// backend keeps each module's types in one place too (domain.go /
// types.go) and pages routinely need to compose several of them.

export interface Department {
  ID: string
  OrgID: string
  Name: string
  LeadMemberID: string | null
}

export interface Role {
  ID: string
  OrgID: string
  Name: string
  IsBuiltin: boolean
}

export interface Member {
  ID: string
  OrgID: string
  DepartmentID: string | null
  RoleID: string
  Name: string
  Email: string | null
  Phone: string | null
  AvatarURL: string | null
  Status: "active" | "pending_review" | "disabled"
}

export interface IdentityConfig {
  Provider: string
  AppID: string
  AppSecret: string
  CallbackPath: string
  Enabled: boolean
}

export interface AuthSettings {
  LocalAccountEnabled: boolean
  LocalAccountRequiresApproval: boolean
}

export interface NotifyChannel {
  Kind: "sms" | "email"
  Provider: string
  Config: Record<string, string>
  Enabled: boolean
}

export interface Provider {
  ID: string
  OrgID: string
  Name: string
  Kind: string
  Config: Record<string, string>
  Status: "active" | "disabled"
}

export interface Model {
  ID: string
  ProviderID: string
  Name: string
  ModelIdentifier: string
  Status: "draft" | "published"
  InputPriceCentsPer1M: number
  OutputPriceCentsPer1M: number
  ContextWindow: number
  // Joined in by both model list endpoints (/api/models and
  // /api/models/published -- see internal/provider/repo.go), so a model
  // picker can show its vendor mark without also fetching /api/providers.
  ProviderKind?: string
}

export interface ProcurementRecord {
  ID: string
  ProviderID: string
  AmountMicroCents: number
  Note: string
  RecordedByMemberID: string
  RecordedAt: string
}

export interface RoutingRule {
  ID: string
  Scope: "global" | "personal"
  OwnerMemberID: string | null
  ConditionLabel: string
  TargetModelID: string
  FallbackModelID: string | null
  CostCeilingMicroCents: number | null
}

export interface VirtualKey {
  ID: string
  Name: string
  SecretPrefix: string
  OwnerType: "member" | "department"
  OwnerMemberID: string | null
  OwnerDepartmentID: string | null
  ModelScope: string[] | null
  BudgetMicroCents: number
  SpentMicroCents: number
  /** Promised to calls that are still in flight; available budget is
   *  Budget - Spent - Reserved. Transient, so the console shows spend
   *  against budget rather than subtracting it. */
  ReservedMicroCents: number
  Status: "active" | "revoked"
  CreatedAt: string
}

// A department pool's live state. Remaining is never stored -- the
// backend recomputes it as Total - Spoken on every read (see
// internal/provider/quota/approval.go), and it may legitimately be
// negative when approvals have overdrawn the pool.
export interface QuotaBalance {
  Total: number
  Spoken: number
  Remaining: number
}

export interface QuotaRequest {
  ID: string
  RequestedByMemberID: string
  ModelID: string | null
  AmountMicroCents: number
  Reason: string
  Status: "pending" | "approved" | "rejected"
  CreatedAt: string
}

export interface ProviderHealth {
  ProviderID: string
  State: "normal" | "circuit_open" | "half_open"
  ConsecutiveFailures: number
}

export interface DLPRule {
  ID: string
  Name: string
  MatchType: "regex_checksum" | "keyword"
  Pattern: string
  Action: "mask" | "block"
  Priority: number
  Enabled: boolean
}

export interface SecurityEvent {
  ID: string
  MemberID: string | null
  VirtualKeyID: string | null
  RuleID: string | null
  Description: string
  ActionTaken: "mask" | "block"
  OccurredAt: string
}

export interface CallLog {
  ID: string
  MemberID: string
  VirtualKeyID: string
  ProviderID: string
  ModelID: string
  RequestID: string
  Status: "success" | "failed"
  LatencyMS: number
  InputTokens: number
  OutputTokens: number
  CostMicroCents: number
  OccurredAt: string
}

export interface OperationAuditLog {
  ID: string
  ActorMemberID: string
  Action: string
  Detail: string
  OccurredAt: string
}
