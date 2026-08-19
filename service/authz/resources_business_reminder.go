package authz

const ResourceBusinessReminder = "business_reminder"

var (
	BusinessReminderRead = Permission{Resource: ResourceBusinessReminder, Action: ActionRead}
	BusinessReminderRun  = Permission{Resource: ResourceBusinessReminder, Action: ActionUpdate}
)

func init() {
	RegisterResource(ResourceDefinition{
		Resource: ResourceBusinessReminder,
		LabelKey: "Business reminders",
		Actions: []ActionDefinition{
			{
				Action:         ActionRead,
				LabelKey:       "Read business reminders",
				DescriptionKey: "View global enterprise project reminder state.",
				DefaultRoles:   []string{BusinessRolePlatformManager, BusinessRoleOperationsReader},
			},
			{
				Action:         ActionUpdate,
				LabelKey:       "Run business reminder scan",
				DescriptionKey: "Queue an on-demand enterprise project reminder scan.",
				DefaultRoles:   []string{BusinessRolePlatformManager},
			},
		},
	})
}
