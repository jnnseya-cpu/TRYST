package aigov

// Workforce returns the manifests of TRYST's agent workforce (09 §5–§6). It is the source
// of truth the registry is loaded from at boot; the tests pin its safety properties.
//
// Member-plane agents run on self-hosted models inside the member platform. Every other
// agent works on telemetry, code, public data or aggregates, and holds no member scope.
func Workforce() []Manifest {
	member := func(id, owner string, lvl Level, budget int, scopes []string, actions map[string]Class) Manifest {
		return Manifest{ID: id, Owner: owner, Level: lvl, Actions: actions, DataScopes: scopes,
			MemberPlane: true, SelfHosted: true, DailyBudget: budget}
	}
	ops := func(id, owner string, lvl Level, budget int, scopes []string, actions map[string]Class) Manifest {
		return Manifest{ID: id, Owner: owner, Level: lvl, Actions: actions, DataScopes: scopes, DailyBudget: budget}
	}
	return []Manifest{
		// Member-facing matching agents (01 §7; 03 §3).
		member("cartographer", "product", L1Recommend, 0, []string{"profile_db"},
			map[string]Class{"propose_intent_vector": Recommend, "read_intake": Read}),
		member("mirror", "ml", L2Reversible, 1_000_000, []string{"event_bus", "feature_store"},
			map[string]Class{"update_revealed_vector": Reversible, "read_events": Read}),
		member("broker", "ml", L2Reversible, 10_000_000, []string{"vector_store", "profile_db"},
			map[string]Class{"compute_slate": Reversible, "read_candidates": Read}),
		member("envoy", "product", L1Recommend, 0, []string{"profile_db"},
			map[string]Class{"negotiate_handshake": Recommend, "propose_outreach": Recommend}),
		member("curtain", "security", L2Reversible, 1_000_000, []string{"profile_db"},
			map[string]Class{"tighten_policy": Reversible, "suppress_exposure": Reversible}),
		member("guardian", "trust_safety", L2Reversible, 100_000, []string{"safety_db", "profile_db"},
			map[string]Class{"hold_interaction": Reversible, "escalate_case": Reversible,
				"remove_member": MemberSafety, "freeze_account": MemberSafety}),
		member("aftercare", "product", L2Reversible, 1_000_000, []string{"profile_db"},
			map[string]Class{"schedule_prompt": Reversible, "record_outcome": Reversible}),
		// Core agents from the AI-OS brief that touch member data (09 §6).
		member("onboarding", "product", L2Reversible, 100_000, []string{"identity_db", "profile_db"},
			map[string]Class{"route_verification_step": Reversible, "explain_step": Recommend}),
		member("risk", "trust_safety", L2Reversible, 100_000, []string{"safety_db", "identity_db"},
			map[string]Class{"score_signup": Recommend, "require_step_up": Reversible, "ban_anchor_match": MemberSafety}),
		member("fraud", "payments", L2Reversible, 50_000, []string{"identity_db"},
			map[string]Class{"score_payment": Recommend, "hold_payment": Reversible, "refund": MoneyMovement}),
		member("support", "support", L1Recommend, 0, []string{"profile_db"},
			map[string]Class{"draft_reply": Recommend, "issue_goodwill_keys": MoneyMovement}),
		member("chief-of-staff", "product", L1Recommend, 0, []string{"profile_db"},
			map[string]Class{"brief_member": Recommend, "suggest_next_step": Recommend}),
		member("compliance", "dpo", L2Reversible, 10_000, []string{"profile_db", "identity_db"},
			map[string]Class{"run_dsr_export": Reversible, "check_retention": Read, "erase_account": Irreversible}),
		member("payment", "payments", L2Reversible, 100_000, []string{"identity_db"},
			map[string]Class{"reconcile_ledger": Reversible, "route_processor": Reversible, "change_price": MoneyMovement}),

		// Growth (outside the member platform).
		ops("herald", "growth", L2Reversible, 200, []string{"public_web", "search_console", "site_analytics_aggregate"},
			map[string]Class{"draft_page": Recommend, "publish_page": Irreversible, "fix_internal_link": Reversible}),
		ops("revenue-analyst", "finance", L1Recommend, 0, []string{"warehouse_aggregates"},
			map[string]Class{"forecast": Recommend, "pricing_experiment_proposal": Recommend}),

		// Self-managing platform layer (09 §7).
		ops("system-health", "sre", L2Reversible, 10_000, []string{"telemetry"},
			map[string]Class{"read_metrics": Read, "open_incident": Reversible, "page_oncall": Reversible}),
		ops("bug-detection", "engineering", L2Reversible, 1_000, []string{"telemetry", "ci"},
			map[string]Class{"file_issue": Reversible, "bisect_regression": Recommend}),
		ops("auto-repair", "sre", L3Runbook, 50, []string{"telemetry", "deploy"},
			map[string]Class{"rollback_release": Runbook, "restart_service": Runbook, "scale_out": Runbook,
				"open_fix_pr": Recommend, "deploy_patch": ProductionChange}),
		ops("infra-optimiser", "sre", L1Recommend, 0, []string{"telemetry", "cloud_billing"},
			map[string]Class{"rightsizing_proposal": Recommend, "apply_rightsizing": ProductionChange}),
		ops("release-manager", "engineering", L3Runbook, 20, []string{"ci", "deploy"},
			map[string]Class{"halt_canary": Runbook, "rollback_release": Runbook, "promote_release": ProductionChange}),
		ops("threat-hunter", "security", L2Reversible, 1_000, []string{"telemetry"},
			map[string]Class{"open_incident": Reversible, "block_ip_range": Reversible}),
		ops("vulnerability", "security", L2Reversible, 200, []string{"ci"},
			map[string]Class{"file_issue": Reversible, "open_dependency_pr": Recommend}),
	}
}
