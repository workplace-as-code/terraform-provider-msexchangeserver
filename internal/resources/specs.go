package resources

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
)

// Parameter names are the ones of the Set-<noun> cmdlet (checked against
// Exchange Server SE), not guesses: a wrong name fails every apply.
// Exchange Online-only parameters (SmtpClientAuthenticationDisabled,
// ReplyAllStorm*, ...) do not exist on Exchange Server.
var objectSpecs = []objectSpec{
	{
		typeName: "retention_policy_tag",
		noun:     "RetentionPolicyTag",
		description: "Manages a retention policy tag (messaging records management), which sets how long items are kept " +
			"and what happens when the age limit is reached. Link tags to mailboxes with `msexchangeserver_retention_policy`",
		notes: "`type` is required to create a tag.",
		fields: []field{
			{
				name: "type", param: "Type", kind: stringField, forceNew: true,
				desc: "Folder the tag applies to: `All` (default policy tag), `Personal` (users apply it themselves), or a default folder " +
					"such as `Inbox`, `SentItems`, `DeletedItems`, `JunkEmail`, `Calendar`, `Tasks`, `RecoverableItems`. " +
					"Changing it creates a new tag.",
			},
			daysAttr("age_limit_for_retention", "AgeLimitForRetention", "Age, in days, at which `retention_action` applies. "+
				"`0` means no age limit, which Exchange only accepts with `retention_enabled = false` (built-in tags such as Never Delete).",
				int64validator.Between(0, 24855)),
			enumAttr("retention_action", "RetentionAction", "Action when the age limit is reached.",
				"DeleteAndAllowRecovery", "PermanentlyDelete", "MoveToArchive", "MarkAsPastRetentionLimit"),
			boolAttr("retention_enabled", "RetentionEnabled", "Whether the tag is applied. `false` keeps it visible but inactive."),
			stringAttr("comment", "Comment", "Comment shown to users in Outlook."),
		},
	},
	{
		typeName:    "transport_config",
		noun:        "TransportConfig",
		singleton:   true,
		edge:        true,
		description: "Manages the organization-wide transport settings",
		fields: []field{
			stringAttr("external_postmaster_address", "ExternalPostmasterAddress", "Sender address of the delivery status notifications sent to external senders."),
			boolAttr("address_book_policy_routing_enabled", "AddressBookPolicyRoutingEnabled",
				"Resolve recipients to the global address list of the sender's address book policy."),
			boolAttr("convert_disclaimer_wrapper_to_eml", "ConvertDisclaimerWrapperToEml",
				"Attach the original message as an .eml file when a transport rule disclaimer cannot be inserted."),
			boolAttr("rfc2231_encoding_enabled", "Rfc2231EncodingEnabled", "Encode attachment names with RFC 2231."),

			// Message limits
			sizeAttr("max_send_size_mb", "MaxSendSize", "Largest message, in megabytes, that a user can send. `0` means unlimited. "+
				"Connectors and mailboxes can set lower limits.", int64validator.Between(0, 2047)),
			sizeAttr("max_receive_size_mb", "MaxReceiveSize", "Largest message, in megabytes, that a user can receive. `0` means unlimited. "+
				"Connectors and mailboxes can set lower limits.", int64validator.Between(0, 2047)),
			limitAttr("max_recipient_envelope_limit", "MaxRecipientEnvelopeLimit",
				"Largest number of recipients in a message, a distribution group counting as one. `0` means unlimited.",
				int64validator.Between(0, 2147483647)),

			// Delivery status notifications
			stringAttr("external_dsn_reporting_authority", "ExternalDsnReportingAuthority", "Domain shown in the machine-readable part of the "+
				"delivery status notifications sent outside the organization. Exchange uses the authoritative domain when it is not set."),
			stringAttr("internal_dsn_reporting_authority", "InternalDsnReportingAuthority", "Domain shown in the machine-readable part of the "+
				"delivery status notifications sent inside the organization. Exchange uses the authoritative domain when it is not set."),
			boolAttr("external_dsn_send_html", "ExternalDsnSendHtml", "Send the delivery status notifications to external senders in HTML (otherwise plain text)."),
			boolAttr("internal_dsn_send_html", "InternalDsnSendHtml", "Send the delivery status notifications to internal senders in HTML (otherwise plain text)."),
			boolAttr("external_delay_dsn_enabled", "ExternalDelayDsnEnabled", "Send delay notifications to external senders."),
			boolAttr("internal_delay_dsn_enabled", "InternalDelayDsnEnabled", "Send delay notifications to internal senders."),
			listAttr("generate_copy_of_dsn_for", "GenerateCopyOfDSNFor", "Enhanced status codes (for example `5.4.8`) of the non-delivery "+
				"reports copied to the Exchange recipient (the postmaster mailbox)."),
			enumAttr("dsn_conversion_mode", "DSNConversionMode", "How delivery status notifications from other messaging systems are converted.",
				"UseExchangeDSNs", "PreserveDSNBody", "DoNotConvert"),
			stringAttr("journaling_report_ndr_to", "JournalingReportNdrTo",
				"Mailbox that receives the journal reports that could not be delivered to the journaling mailbox."),

			// Transport servers and TLS
			listAttr("internal_smtp_servers", "InternalSMTPServers", "IP addresses or ranges of the internal SMTP servers, ignored by "+
				"Sender ID and connection filtering (for example a load balancer or a relay in front of Exchange)."),
			listAttr("tls_send_domain_secure_list", "TLSSendDomainSecureList", "Domains to which mail is sent with mutual TLS authentication (domain security)."),
			listAttr("tls_receive_domain_secure_list", "TLSReceiveDomainSecureList", "Domains from which mail is received with mutual TLS authentication (domain security)."),
			enumAttr("header_promotion_mode_setting", "HeaderPromotionModeSetting", "Whether named properties are created for the custom X-headers of received messages.",
				"NoCreate", "MayCreate", "MustCreate"),
			boolAttr("clear_categories", "ClearCategories", "Remove the Outlook categories of messages during content conversion."),
			boolAttr("voicemail_journaling_enabled", "VoicemailJournalingEnabled", "Journal the voice mail and missed call notifications."),

			// High availability
			boolAttr("shadow_redundancy_enabled", "ShadowRedundancyEnabled", "Keep a shadow copy of every message on another transport server until it is delivered."),
			boolAttr("reject_message_on_shadow_failure", "RejectMessageOnShadowFailure", "Reject a message when no shadow copy can be made."),
			enumAttr("shadow_message_preference_setting", "ShadowMessagePreferenceSetting", "Where the shadow copy is made: a server of the "+
				"same Active Directory site, of another site, or of another site when one is available.",
				"LocalOnly", "RemoteOnly", "PreferRemote"),
			minutesAttr("safety_net_hold_time_minutes", "SafetyNetHoldTime", "Minutes during which Safety Net keeps a copy of the delivered "+
				"messages, for redelivery after a database failover: `2880` (2 days) by default on Mailbox servers, `10` on Edge Transport servers.",
				int64validator.Between(1, 35791394)),
		},
	},
}
