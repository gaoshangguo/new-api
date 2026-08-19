package authz

const (
	ResourceBusinessCompany            = "business_company"
	ResourceBusinessProject            = "business_project"
	ResourceBusinessCustomerAssignment = "business_customer_assignment"
	ResourceBusinessLedger             = "business_ledger"
	ResourceFinance                    = "finance"
	ResourceBusinessManualCredit       = ResourceFinance
	ResourceBusinessReport             = "business_report"
	ResourceBusinessSales              = "business_sales"
	ResourceBusinessFollowUp           = "business_follow_up"
	ResourceBusinessOperations         = "business_operations"

	ActionCreate  = "create"
	ActionUpdate  = "update"
	ActionApprove = "approve"
)

var (
	BusinessCompanyRead   = Permission{Resource: ResourceBusinessCompany, Action: ActionRead}
	BusinessCompanyCreate = Permission{Resource: ResourceBusinessCompany, Action: ActionCreate}
	BusinessCompanyUpdate = Permission{Resource: ResourceBusinessCompany, Action: ActionUpdate}

	BusinessProjectRead   = Permission{Resource: ResourceBusinessProject, Action: ActionRead}
	BusinessProjectCreate = Permission{Resource: ResourceBusinessProject, Action: ActionCreate}
	BusinessProjectUpdate = Permission{Resource: ResourceBusinessProject, Action: ActionUpdate}

	BusinessCustomerAssignmentRead   = Permission{Resource: ResourceBusinessCustomerAssignment, Action: ActionRead}
	BusinessCustomerAssignmentCreate = Permission{Resource: ResourceBusinessCustomerAssignment, Action: ActionCreate}
	BusinessCustomerAssignmentUpdate = Permission{Resource: ResourceBusinessCustomerAssignment, Action: ActionUpdate}

	BusinessLedgerRead = Permission{Resource: ResourceBusinessLedger, Action: ActionRead}

	BusinessManualCreditRead    = Permission{Resource: ResourceBusinessManualCredit, Action: ActionRead}
	BusinessManualCreditCreate  = Permission{Resource: ResourceBusinessManualCredit, Action: ActionCreate}
	BusinessManualCreditApprove = Permission{Resource: ResourceBusinessManualCredit, Action: ActionApprove}
	BusinessFinanceReconcile    = Permission{Resource: ResourceBusinessManualCredit, Action: ActionUpdate}

	BusinessReportRead = Permission{Resource: ResourceBusinessReport, Action: ActionRead}
	BusinessSalesRead  = Permission{Resource: ResourceBusinessSales, Action: ActionRead}

	BusinessFollowUpRead   = Permission{Resource: ResourceBusinessFollowUp, Action: ActionRead}
	BusinessFollowUpCreate = Permission{Resource: ResourceBusinessFollowUp, Action: ActionCreate}
	BusinessFollowUpUpdate = Permission{Resource: ResourceBusinessFollowUp, Action: ActionUpdate}

	BusinessOperationsRead   = Permission{Resource: ResourceBusinessOperations, Action: ActionRead}
	BusinessOperationsUpdate = Permission{Resource: ResourceBusinessOperations, Action: ActionUpdate}
)

func init() {
	RegisterResource(ResourceDefinition{
		Resource: ResourceBusinessCompany,
		LabelKey: "Enterprise customers",
		Actions: []ActionDefinition{
			{Action: ActionRead, LabelKey: "Read enterprise customers", DescriptionKey: "View enterprise customer profiles.", DefaultRoles: []string{BusinessRolePlatformManager}},
			{Action: ActionCreate, LabelKey: "Create enterprise customers", DescriptionKey: "Create enterprise customer profiles.", DefaultRoles: []string{BusinessRolePlatformManager}},
			{Action: ActionUpdate, LabelKey: "Update enterprise customers", DescriptionKey: "Update enterprise customer profiles.", DefaultRoles: []string{BusinessRolePlatformManager}},
		},
	})
	RegisterResource(ResourceDefinition{
		Resource: ResourceBusinessProject,
		LabelKey: "Enterprise projects",
		Actions: []ActionDefinition{
			{Action: ActionRead, LabelKey: "Read enterprise projects", DescriptionKey: "View enterprise projects and their controls.", DefaultRoles: []string{BusinessRolePlatformManager}},
			{Action: ActionCreate, LabelKey: "Create enterprise projects", DescriptionKey: "Create enterprise projects.", DefaultRoles: []string{BusinessRolePlatformManager}},
			{Action: ActionUpdate, LabelKey: "Update enterprise projects", DescriptionKey: "Update enterprise project controls.", DefaultRoles: []string{BusinessRolePlatformManager}},
		},
	})
	RegisterResource(ResourceDefinition{
		Resource: ResourceBusinessCustomerAssignment,
		LabelKey: "Customer ownership",
		Actions: []ActionDefinition{
			{Action: ActionRead, LabelKey: "Read customer ownership", DescriptionKey: "View customer ownership assignments.", DefaultRoles: []string{BusinessRolePlatformManager}},
			{Action: ActionCreate, LabelKey: "Create customer ownership", DescriptionKey: "Assign a customer to a sales owner.", DefaultRoles: []string{BusinessRolePlatformManager}},
			{Action: ActionUpdate, LabelKey: "Update customer ownership", DescriptionKey: "Change customer ownership with history.", DefaultRoles: []string{BusinessRolePlatformManager}},
		},
	})
	RegisterResource(ResourceDefinition{
		Resource: ResourceBusinessLedger,
		LabelKey: "Balance ledger",
		Actions: []ActionDefinition{
			{Action: ActionRead, LabelKey: "Read balance ledger", DescriptionKey: "View immutable balance ledger records.", DefaultRoles: []string{BusinessRolePlatformManager, BusinessRoleFinanceEntry, BusinessRoleFinanceApprover}},
		},
	})
	RegisterResource(ResourceDefinition{
		Resource: ResourceBusinessManualCredit,
		LabelKey: "Manual credit review",
		Actions: []ActionDefinition{
			{Action: ActionRead, LabelKey: "Read manual credit requests", DescriptionKey: "View manual credit requests and their review state.", DefaultRoles: []string{BusinessRolePlatformManager, BusinessRoleFinanceEntry, BusinessRoleFinanceApprover}},
			{Action: ActionCreate, LabelKey: "Create manual credit requests", DescriptionKey: "Submit a manual credit request for review.", DefaultRoles: []string{BusinessRoleFinanceEntry}},
			{Action: ActionApprove, LabelKey: "Approve manual credit requests", DescriptionKey: "Approve or reject manual credit requests.", DefaultRoles: []string{BusinessRoleFinanceApprover}},
			{Action: ActionUpdate, LabelKey: "Reconcile project budget exceptions", DescriptionKey: "Resolve an audited project budget reconciliation exception.", DefaultRoles: []string{BusinessRolePlatformManager, BusinessRoleFinanceApprover}},
		},
	})
	RegisterResource(ResourceDefinition{
		Resource: ResourceBusinessReport,
		LabelKey: "Business reports",
		Actions: []ActionDefinition{
			{Action: ActionRead, LabelKey: "Read business reports", DescriptionKey: "View global business usage and operations reports.", DefaultRoles: []string{BusinessRolePlatformManager, BusinessRoleOperationsReader}},
		},
	})
	RegisterResource(ResourceDefinition{
		Resource: ResourceBusinessSales,
		LabelKey: "Sales workspace",
		Actions: []ActionDefinition{
			{Action: ActionRead, LabelKey: "Read assigned customers", DescriptionKey: "View only the sales user's assigned customer data.", DefaultRoles: []string{BusinessRolePlatformManager, BusinessRoleSalesSupervisor}},
		},
	})
	RegisterResource(ResourceDefinition{
		Resource: ResourceBusinessFollowUp,
		LabelKey: "Sales follow-ups",
		Actions: []ActionDefinition{
			{Action: ActionRead, LabelKey: "Read sales follow-ups", DescriptionKey: "View follow-up items for assigned customers.", DefaultRoles: []string{BusinessRolePlatformManager, BusinessRoleSalesSupervisor}},
			{Action: ActionCreate, LabelKey: "Create sales follow-ups", DescriptionKey: "Create a follow-up item for an assigned customer.", DefaultRoles: []string{BusinessRolePlatformManager, BusinessRoleSalesSupervisor}},
			{Action: ActionUpdate, LabelKey: "Update sales follow-ups", DescriptionKey: "Update a follow-up item for an assigned customer.", DefaultRoles: []string{BusinessRolePlatformManager, BusinessRoleSalesSupervisor}},
		},
	})
	RegisterResource(ResourceDefinition{
		Resource: ResourceBusinessOperations,
		LabelKey: "Operations status",
		Actions: []ActionDefinition{
			{Action: ActionRead, LabelKey: "Read operations status", DescriptionKey: "View channel and platform health without upstream secrets.", DefaultRoles: []string{BusinessRolePlatformManager, BusinessRoleOperationsReader}},
			{Action: ActionUpdate, LabelKey: "Update operations status", DescriptionKey: "Publish maintenance and known-incident status updates.", DefaultRoles: []string{BusinessRolePlatformManager}},
		},
	})
}
