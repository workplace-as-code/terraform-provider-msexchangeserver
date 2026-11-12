package resources

import (
	"fmt"

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
	{
		typeName:    "admin_audit_log_config",
		noun:        "AdminAuditLogConfig",
		singleton:   true,
		description: "Manages administrator audit logging: which cmdlets run by administrators are recorded, and for how long",
		notes: "Lowering `admin_audit_log_age_limit_days` deletes the older entries. A change can take up to 60 " +
			"minutes to apply where an Exchange Management Shell is open. Mailbox audit logging is a setting of each " +
			"mailbox: the provider does not manage it.",
		fields: []field{
			boolAttr("admin_audit_log_enabled", "AdminAuditLogEnabled", "Record the cmdlets run by administrators."),
			daysAttr("admin_audit_log_age_limit_days", "AdminAuditLogAgeLimit", "Days the entries are kept.",
				int64validator.Between(1, 24855)),
			setAttr("admin_audit_log_cmdlets", "AdminAuditLogCmdlets",
				"Cmdlets recorded, with wildcards (`*Mailbox*`). `[\"*\"]` records every cmdlet."),
			setAttr("admin_audit_log_parameters", "AdminAuditLogParameters",
				"Parameters recorded, with wildcards. `[\"*\"]` records every parameter."),
			setAttr("admin_audit_log_excluded_cmdlets", "AdminAuditLogExcludedCmdlets",
				"Cmdlets never recorded, with wildcards. Exchange refuses `*` alone."),
			enumAttr("log_level", "LogLevel", "`Verbose` also records the old and new values of the changed properties.",
				"None", "Verbose"),
			boolAttr("test_cmdlet_logging_enabled", "TestCmdletLoggingEnabled",
				"Record the Test- cmdlets too. They produce many entries."),
		},
	},
	{
		typeName:    "send_connector",
		noun:        "SendConnector",
		script:      "send_connector.ps1",
		description: "Manages a send connector, which routes outgoing mail for some domains through DNS or smart hosts",
		notes: "`address_spaces` is required to create a connector, which is created with the `Custom` usage. The connectors that EdgeSync creates (`EdgeSync - ...`) " +
			"can be read or imported, but EdgeSync overwrites any change. The provider never sends smart host credentials: " +
			"the password would be stored in the Terraform state.",
		fields: []field{
			setAttr("address_spaces", "AddressSpaces", "Domains the connector routes mail to, as `contoso.com` or `contoso.com;10` "+
				"with a cost (1 by default). `*` routes all external mail; `*.contoso.com` includes the subdomains."),
			setAttr("smart_hosts", "SmartHosts", "Host names or IP addresses mail is sent to. Leave empty and set "+
				"`dns_routing_enabled` to route with DNS MX records."),
			boolAttr("dns_routing_enabled", "DNSRoutingEnabled", "Route mail with DNS MX records instead of smart hosts."),
			setAttr("source_transport_servers", "SourceTransportServers", "Mailbox or Edge servers that send mail through the connector."),
			int64Attr("port", "Port", "TCP port of the destination.", int64validator.Between(1, 65535)),
			stringAttr("fqdn", "Fqdn", "Name the servers give in the HELO or EHLO command. Empty uses each server's FQDN."),
			sizeAttr("max_message_size_mb", "MaxMessageSize", "Largest message, in megabytes, the connector sends. `0` means unlimited."),
			boolAttr("enabled", "Enabled", "Whether the connector is used."),
			boolAttr("frontend_proxy_enabled", "FrontendProxyEnabled", "Send outgoing mail through the Front End Transport service."),
			boolAttr("require_tls", "RequireTLS", "Require TLS: mail waits in the queue when the destination does not offer it."),
			enumAttr("tls_auth_level", "TlsAuthLevel", "Check made on the certificate of the destination when `require_tls` is `true`.",
				"EncryptionOnly", "CertificateValidation", "DomainValidation"),
			stringAttr("tls_domain", "TlsDomain", "Domain the certificate of the destination must match with `DomainValidation`."),
			stringAttr("tls_certificate_name", "TlsCertificateName", "Certificate the servers present, as `<I>issuer<S>subject`."),
			enumAttr("smart_host_auth_mechanism", "SmartHostAuthMechanism", "Authentication with the smart hosts. `BasicAuth` and "+
				"`BasicAuthRequireTLS` need a user name and password, which the provider does not manage: set them with "+
				"`Set-SendConnector -AuthenticationCredential` first. `ExchangeServer` is used by the connectors of EdgeSync.",
				"None", "BasicAuth", "BasicAuthRequireTLS", "ExchangeServer", "ExternalAuthoritative"),
			enumAttr("protocol_logging_level", "ProtocolLoggingLevel", "SMTP protocol logging.", "None", "Verbose"),
			stringAttr("comment", "Comment", "Free text about the connector."),
		},
	},
	{
		typeName: "receive_connector",
		noun:     "ReceiveConnector",
		script:   "receive_connector.ps1",
		edge:     true,
		description: "Manages a receive connector of an Exchange server, which accepts SMTP connections on given addresses " +
			"and ports from given IP ranges",
		notes: "`name` is the identity of the connector, `<server>\\<name>`, with the server's short name, as " +
			"`Get-ReceiveConnector` shows it: changing it creates a new connector on that server. `bindings` is required to " +
			"create a connector, which is created with the `Custom` usage: set its `permission_groups` and `auth_mechanism`. For an application that relays mail, prefer `permission_groups = [\"ExchangeServers\"]` with " +
			"`auth_mechanism = [\"ExternalAuthoritative\"]` on narrow `remote_ip_ranges`: it needs no Active Directory permission.",
		fields: []field{
			enumAttr("transport_role", "TransportRole", "Transport service of the connector on a Mailbox server.",
				"FrontendTransport", "HubTransport"),
			setAttr("bindings", "Bindings", "Local addresses and ports, as `0.0.0.0:25` or `[::]:25`."),
			setAttr("remote_ip_ranges", "RemoteIPRanges", "Remote IP addresses, ranges (`10.0.0.1-10.0.0.9`) or CIDR blocks the connector accepts."),
			setAttr("permission_groups", "PermissionGroups", "Groups allowed to use the connector: `AnonymousUsers`, `ExchangeUsers`, "+
				"`ExchangeServers`, `ExchangeLegacyServers`, `Partners`, `Custom`."),
			setAttr("auth_mechanism", "AuthMechanism", "Authentication offered: `None`, `Tls`, `Integrated`, `BasicAuth`, "+
				"`BasicAuthRequireTLS`, `ExchangeServer`, `ExternalAuthoritative`."),
			boolAttr("require_tls", "RequireTLS", "Require TLS for every connection."),
			stringAttr("fqdn", "Fqdn", "Name the connector gives in its banner and EHLO response."),
			stringAttr("banner", "Banner", "SMTP banner, starting with `220`. Empty uses the Exchange default."),
			sizeAttr("max_message_size_mb", "MaxMessageSize", "Largest message, in megabytes, the connector accepts. `0` means unlimited."),
			stringAttr("tls_certificate_name", "TlsCertificateName", "Certificate the connector presents, as `<I>issuer<S>subject`."),
			boolAttr("enabled", "Enabled", "Whether the connector accepts connections."),
			enumAttr("protocol_logging_level", "ProtocolLoggingLevel", "SMTP protocol logging.", "None", "Verbose"),
			stringAttr("comment", "Comment", "Free text about the connector."),
		},
	},
	{
		typeName: "mailbox_database",
		noun:     "MailboxDatabase",
		adopted:  true,
		description: "Manages the quotas, retention and logging settings of a mailbox database. Mailboxes without " +
			"quotas of their own use the database's",
		notes: "`name` is the database name, unique in the organization. Databases are created and removed with the " +
			"Exchange tools: the provider never mounts, dismounts or removes them. Sizes are in megabytes, `0` meaning unlimited.",
		fields: []field{
			sizeAttr("issue_warning_quota_mb", "IssueWarningQuota", "Mailbox size at which users are warned."),
			sizeAttr("prohibit_send_quota_mb", "ProhibitSendQuota", "Mailbox size at which users can no longer send."),
			sizeAttr("prohibit_send_receive_quota_mb", "ProhibitSendReceiveQuota", "Mailbox size at which the mailbox can no longer send or receive."),
			sizeAttr("recoverable_items_warning_quota_mb", "RecoverableItemsWarningQuota", "Size of the Recoverable Items folder at which "+
				"Exchange logs a warning."),
			sizeAttr("recoverable_items_quota_mb", "RecoverableItemsQuota", "Largest size of the Recoverable Items folder."),
			sizeAttr("calendar_logging_quota_mb", "CalendarLoggingQuota", "Largest size of the calendar log in the Recoverable Items folder."),
			daysAttr("deleted_item_retention_days", "DeletedItemRetention", "Days deleted items stay recoverable.",
				int64validator.Between(0, 24855)),
			daysAttr("mailbox_retention_days", "MailboxRetention", "Days a deleted mailbox is kept in the database.",
				int64validator.Between(0, 24855)),
			boolAttr("retain_deleted_items_until_backup", "RetainDeletedItemsUntilBackup",
				"Keep deleted items and mailboxes until the database is backed up, even after their retention period."),
			boolAttr("circular_logging_enabled", "CircularLoggingEnabled", "Overwrite the transaction logs once their changes are "+
				"in the database. Outside a database availability group, the change applies after the database is next mounted."),
			boolAttr("is_excluded_from_provisioning", "IsExcludedFromProvisioning",
				"Exclude the database from the automatic placement of new mailboxes."),
			boolAttr("mount_at_startup", "MountAtStartup", "Mount the database when the Information Store service starts."),
			stringAttr("offline_address_book", "OfflineAddressBook", "Offline address book of the mailboxes of the database, by name."),
		},
	},
	{
		typeName:    "content_filter_config",
		noun:        "ContentFilterConfig",
		singleton:   true,
		edgeOnly:    true,
		description: "Manages the content filter agent: what happens to a message according to its spam confidence level (SCL)",
		notes:       edgeNote,
		fields: []field{
			boolAttr("enabled", "Enabled", "Whether the agent filters messages."),
			boolAttr("external_mail_enabled", "ExternalMailEnabled", "Filter messages from outside the organization."),
			boolAttr("internal_mail_enabled", "InternalMailEnabled", "Filter messages from authenticated partners and internal servers."),
			boolAttr("scl_reject_enabled", "SCLRejectEnabled", "Reject the messages that reach `scl_reject_threshold`."),
			int64Attr("scl_reject_threshold", "SCLRejectThreshold", "SCL from which messages are rejected.", int64validator.Between(0, 9)),
			boolAttr("scl_delete_enabled", "SCLDeleteEnabled", "Delete the messages that reach `scl_delete_threshold`."),
			int64Attr("scl_delete_threshold", "SCLDeleteThreshold", "SCL from which messages are deleted.", int64validator.Between(0, 9)),
			boolAttr("scl_quarantine_enabled", "SCLQuarantineEnabled", "Send the messages that reach `scl_quarantine_threshold` to the quarantine mailbox."),
			int64Attr("scl_quarantine_threshold", "SCLQuarantineThreshold", "SCL from which messages are quarantined.", int64validator.Between(0, 9)),
			stringAttr("quarantine_mailbox", "QuarantineMailbox", "Address of the quarantine mailbox."),
			stringAttr("rejection_response", "RejectionResponse", "Text of the rejection, up to 240 characters."),
			setAttr("bypassed_senders", "BypassedSenders", "Senders whose messages are never filtered."),
			setAttr("bypassed_sender_domains", "BypassedSenderDomains", "Sender domains whose messages are never filtered."),
			setAttr("bypassed_recipients", "BypassedRecipients", "Recipients whose messages are never filtered."),
		},
	},
	{
		typeName:    "sender_filter_config",
		noun:        "SenderFilterConfig",
		singleton:   true,
		edgeOnly:    true,
		description: "Manages the sender filter agent, which blocks given senders and domains",
		notes:       edgeNote,
		fields: []field{
			boolAttr("enabled", "Enabled", "Whether the agent filters messages."),
			boolAttr("external_mail_enabled", "ExternalMailEnabled", "Filter messages from outside the organization."),
			boolAttr("internal_mail_enabled", "InternalMailEnabled", "Filter messages from authenticated partners and internal servers."),
			setAttr("blocked_senders", "BlockedSenders", "Blocked sender addresses."),
			setAttr("blocked_domains", "BlockedDomains", "Blocked sender domains, without their subdomains."),
			setAttr("blocked_domains_and_subdomains", "BlockedDomainsAndSubdomains", "Blocked sender domains, with their subdomains."),
			enumAttr("action", "Action", "What happens to a message from a blocked sender.", "Reject", "StampStatus"),
			boolAttr("blank_sender_blocking_enabled", "BlankSenderBlockingEnabled", "Block the messages without a sender."),
		},
	},
	{
		typeName:    "recipient_filter_config",
		noun:        "RecipientFilterConfig",
		singleton:   true,
		edgeOnly:    true,
		description: "Manages the recipient filter agent, which blocks messages to given or unknown recipients",
		notes:       edgeNote,
		fields: []field{
			boolAttr("enabled", "Enabled", "Whether the agent filters messages."),
			boolAttr("external_mail_enabled", "ExternalMailEnabled", "Filter messages from outside the organization."),
			boolAttr("internal_mail_enabled", "InternalMailEnabled", "Filter messages from authenticated partners and internal servers."),
			boolAttr("block_list_enabled", "BlockListEnabled", "Block the messages to `blocked_recipients`."),
			setAttr("blocked_recipients", "BlockedRecipients", "Recipient addresses that never receive mail from outside."),
			boolAttr("recipient_validation_enabled", "RecipientValidationEnabled", "Reject the messages to recipients that do not exist."),
		},
	},
	{
		typeName:    "ip_allow_list_config",
		noun:        "IPAllowListConfig",
		singleton:   true,
		edgeOnly:    true,
		description: "Manages the IP allow list of the connection filter agent",
		notes:       edgeNote,
		fields: []field{
			boolAttr("enabled", "Enabled", "Whether the IP allow list is used."),
			boolAttr("external_mail_enabled", "ExternalMailEnabled", "Use it for connections from outside the organization."),
			boolAttr("internal_mail_enabled", "InternalMailEnabled", "Use it for authenticated partners and internal servers."),
		},
	},
	{
		typeName:    "ip_block_list_config",
		noun:        "IPBlockListConfig",
		singleton:   true,
		edgeOnly:    true,
		description: "Manages the IP block list of the connection filter agent",
		notes:       edgeNote,
		fields: []field{
			boolAttr("enabled", "Enabled", "Whether the IP block list is used."),
			boolAttr("external_mail_enabled", "ExternalMailEnabled", "Use it for connections from outside the organization."),
			boolAttr("internal_mail_enabled", "InternalMailEnabled", "Use it for authenticated partners and internal servers."),
			stringAttr("static_entry_rejection_response", "StaticEntryRejectionResponse",
				"Rejection text for the entries added by an administrator, up to 240 characters."),
			stringAttr("machine_entry_rejection_response", "MachineEntryRejectionResponse",
				"Rejection text for the entries added by sender reputation, up to 240 characters."),
		},
	},
	{
		typeName:    "ip_block_list_providers_config",
		noun:        "IPBlockListProvidersConfig",
		singleton:   true,
		edgeOnly:    true,
		description: "Manages how the connection filter agent uses the IP block list providers (DNS block lists)",
		notes:       edgeNote,
		fields: []field{
			boolAttr("enabled", "Enabled", "Whether the providers are queried."),
			boolAttr("external_mail_enabled", "ExternalMailEnabled", "Query them for connections from outside the organization."),
			boolAttr("internal_mail_enabled", "InternalMailEnabled", "Query them for authenticated partners and internal servers."),
			setAttr("bypassed_recipients", "BypassedRecipients", "Recipients whose messages are never checked against the providers."),
		},
	},
	ipListEntrySpec("ip_allow_list_entry", "IPAllowListEntry", "allow", "always accepted"),
	ipListEntrySpec("ip_block_list_entry", "IPBlockListEntry", "block", "always rejected"),
	{
		typeName:    "ip_block_list_provider",
		noun:        "IPBlockListProvider",
		script:      "ip_block_list_provider.ps1",
		edgeOnly:    true,
		description: "Manages an IP block list provider: a DNS block list that the connection filter agent queries",
		notes:       "`lookup_domain` is required to create a provider. " + edgeNote,
		fields: []field{
			stringAttr("lookup_domain", "LookupDomain", "DNS zone of the block list, for example `zen.spamhaus.org`."),
			boolAttr("enabled", "Enabled", "Whether the provider is queried."),
			int64Attr("priority", "Priority", "Order in which the providers are queried, 1 first.", int64validator.AtLeast(1)),
			stringAttr("rejection_response", "RejectionResponse", "Rejection text, up to 240 characters."),
			boolAttr("any_match", "AnyMatch", "Block when the lookup returns any address."),
			stringAttr("bitmask_match", "BitmaskMatch", "Block when the returned address matches this bit mask, such as `127.0.0.1`."),
			setAttr("ip_addresses_match", "IPAddressesMatch", "Block when the lookup returns one of these addresses."),
		},
	},
	{
		typeName:    "address_rewrite_entry",
		noun:        "AddressRewriteEntry",
		edgeOnly:    true,
		description: "Manages an address rewrite entry, which rewrites the addresses of messages leaving or entering the organization",
		notes: "`internal_address` and `external_address` are required to create an entry. " +
			"Address rewriting runs on Edge Transport servers only.",
		fields: []field{
			stringAttr("internal_address", "InternalAddress", "Address or domain inside the organization, such as `contoso.com` or `*.contoso.com`."),
			stringAttr("external_address", "ExternalAddress", "Address or domain the messages show outside."),
			boolAttr("outbound_only", "OutboundOnly", "Rewrite outgoing messages only. Required with a wildcard `internal_address`."),
			setAttr("exception_list", "ExceptionList", "Subdomains of a wildcard `internal_address` that are not rewritten."),
		},
	},
	virtualDirectorySpec("owa_virtual_directory", "OwaVirtualDirectory", "Outlook on the web (OWA)", "owa",
		withMethods(formsAuthFields())...),
	virtualDirectorySpec("ecp_virtual_directory", "EcpVirtualDirectory", "Exchange admin center (ECP)", "ecp",
		withMethods(formsAuthFields())...),
	virtualDirectorySpec("web_services_virtual_directory", "WebServicesVirtualDirectory", "Exchange Web Services (EWS)", "EWS",
		withMethods([]field{basicAuthField, windowsAuthField, digestAuthField, oauthField,
			boolAttr("ws_security_authentication", "WSSecurityAuthentication", "WS-Security authentication in IIS."),
		})...),
	virtualDirectorySpec("activesync_virtual_directory", "ActiveSyncVirtualDirectory", "Exchange ActiveSync",
		"Microsoft-Server-ActiveSync",
		boolAttr("basic_auth_enabled", "BasicAuthEnabled", "Basic authentication in IIS."),
		boolAttr("windows_auth_enabled", "WindowsAuthEnabled", "Integrated Windows authentication in IIS."),
		enumAttr("client_cert_auth", "ClientCertAuth", "Client certificate authentication.", "Ignore", "Accepted", "Required"),
		extendedProtectionField),
	virtualDirectorySpec("oab_virtual_directory", "OabVirtualDirectory", "offline address book (OAB)", "OAB",
		withMethods([]field{basicAuthField, windowsAuthField, oauthField,
			boolAttr("require_ssl", "RequireSSL", "Require SSL in IIS."),
		})...),
	virtualDirectorySpec("mapi_virtual_directory", "MapiVirtualDirectory", "MAPI over HTTP", "mapi",
		withMethods([]field{
			setEnumAttr("iis_authentication_methods", "IISAuthenticationMethods",
				"Authentication methods of the IIS directory.", "Basic", "Ntlm", "Negotiate", "OAuth"),
		})...),
	{
		typeName: "outlook_anywhere",
		noun:     "OutlookAnywhere",
		script:   "virtual_directory.ps1",
		adopted:  true,
		description: "Manages Outlook Anywhere (RPC over HTTP, the `/Rpc` virtual directory) of an Exchange server: " +
			"its host names and authentication settings",
		notes: "`name` is the server name or FQDN, for the directory of the Default Web Site, or its identity as " +
			"`Get-OutlookAnywhere` shows it, such as `EX01\\Rpc (Default Web Site)`. " +
			"Authentication settings are read from IIS: a plan fails when the server's IIS does not answer.",
		fields: []field{
			stringAttr("internal_hostname", "InternalHostname",
				"Host name used inside the network, such as `mail.contoso.com`. An empty string clears it."),
			stringAttr("external_hostname", "ExternalHostname",
				"Host name used from the internet. An empty string clears it."),
			enumAttr("internal_client_authentication_method", "InternalClientAuthenticationMethod",
				"Authentication method of clients inside the network.", "Basic", "Ntlm", "Negotiate"),
			enumAttr("external_client_authentication_method", "ExternalClientAuthenticationMethod",
				"Authentication method of clients on the internet.", "Basic", "Ntlm", "Negotiate"),
			setEnumAttr("iis_authentication_methods", "IISAuthenticationMethods",
				"Authentication methods of the IIS directory.", "Basic", "Ntlm", "Negotiate"),
			boolAttr("internal_clients_require_ssl", "InternalClientsRequireSsl", "Require SSL for clients inside the network."),
			boolAttr("external_clients_require_ssl", "ExternalClientsRequireSsl",
				"Require SSL for clients on the internet. Exchange ignores it while `external_hostname` is empty."),
			boolAttr("ssl_offloading", "SSLOffloading",
				"A load balancer in front of the server decrypts SSL: the directory does not require it."),
		},
	},
	{
		typeName: "transport_service",
		noun:     "TransportService",
		script:   "server_settings.ps1",
		adopted:  true,
		listBy:   "Name",
		edge:     true,
		description: "Manages the log settings of the Transport service of an Exchange server: message tracking, " +
			"connectivity and SMTP protocol logs",
		notes: "`name` is the server name or FQDN. With the Mailbox role, `name` is a Mailbox server: manage an Edge " +
			"Transport server with a provider whose role is Edge. " + protocolLogNote + " On a Mailbox server, the " +
			"message tracking directory limit applies to the logs of the Transport service and to those of the Mailbox " +
			"Transport service, each: they can take twice the size.",
		fields: append([]field{
			boolAttr("message_tracking_log_enabled", "MessageTrackingLogEnabled", "Record message tracking logs."),
			pathAttr("message_tracking_log_path", "MessageTrackingLogPath", "Folder of the message tracking logs."),
			logAgeAttr("message_tracking_log_max_age_days", "MessageTrackingLogMaxAge", maxAgeDesc),
			sizeAttr("message_tracking_log_max_directory_size_mb", "MessageTrackingLogMaxDirectorySize",
				"Largest size, in megabytes, of the message tracking folder. `0` means unlimited."),
			sizeAttr("message_tracking_log_max_file_size_mb", "MessageTrackingLogMaxFileSize",
				"Largest size, in megabytes, of a message tracking log file.", int64validator.Between(1, 4096)),
		}, transportLogFields("None")...),
	},
	{
		typeName: "frontend_transport_service",
		noun:     "FrontendTransportService",
		script:   "server_settings.ps1",
		adopted:  true,
		listBy:   "Name",
		description: "Manages the log settings of the Front End Transport service of an Exchange server: connectivity " +
			"and SMTP protocol logs",
		notes:  "`name` is the server name or FQDN. " + protocolLogNote,
		fields: transportLogFields("Verbose"),
	},
	popImapSpec("pop_settings", "PopSettings", "POP3", "Pop3"),
	popImapSpec("imap_settings", "ImapSettings", "IMAP4", "Imap4"),
	{
		typeName: "client_access_service",
		noun:     "ClientAccessService",
		script:   "virtual_directory.ps1",
		adopted:  true,
		description: "Manages the Autodiscover settings of the Client Access services of an Exchange server: the URL that " +
			"domain-joined Outlook clients find in Active Directory",
		notes: "`name` is the server name or FQDN.",
		fields: []field{
			stringAttr("autodiscover_service_internal_uri", "AutoDiscoverServiceInternalUri",
				"Autodiscover URL published in Active Directory (service connection point), for example "+
					"`https://autodiscover.contoso.com/Autodiscover/Autodiscover.xml`. An empty string clears it."),
		},
	},
}

// popImapSpec describes the POP3 or IMAP4 settings of a server (Set-PopSettings,
// Set-ImapSettings): sign-in, certificate, bindings and connection limits.
func popImapSpec(typeName, noun, protocol, service string) objectSpec {
	return objectSpec{
		typeName: typeName,
		noun:     noun,
		script:   "server_settings.ps1",
		adopted:  true,
		listBy:   "Name",
		description: fmt.Sprintf("Manages the %s settings of an Exchange server: sign-in, certificate, bindings and "+
			"connection limits", protocol),
		notes: fmt.Sprintf("`name` is the server name or FQDN. Exchange applies a change once the Microsoft Exchange %s "+
			"services of the server restart, which the provider does not do. The certificate is bound to %s with "+
			"`msexchangeserver_exchange_certificate_services`.", service, protocol),
		fields: []field{
			enumAttr("login_type", "LoginType", "Sign-in method. `SecureLogin`, the Exchange default, accepts a "+
				"clear-text password only on an encrypted connection.",
				"PlainTextLogin", "PlainTextAuthentication", "SecureLogin"),
			stringAttr("x509_certificate_name", "X509CertificateName",
				"Host name of the certificate used for SSL and TLS, such as `mail.contoso.com`."),
			setAttr("internal_connection_settings", "InternalConnectionSettings",
				"Host names, ports and encryption shown to clients inside the network, as `mail.contoso.com:995:SSL`."),
			setAttr("external_connection_settings", "ExternalConnectionSettings",
				"Host names, ports and encryption shown to clients on the internet, as `mail.contoso.com:995:SSL`."),
			setAttr("ssl_bindings", "SSLBindings", "Addresses and ports of the SSL connections, as `0.0.0.0:995`."),
			setAttr("unencrypted_or_tls_bindings", "UnencryptedOrTLSBindings",
				"Addresses and ports of the unencrypted and TLS (STARTTLS) connections, as `0.0.0.0:110`."),
			int64Attr("max_connections", "MaxConnections", "Largest number of connections to the server.",
				int64validator.Between(1, 2147483647)),
			int64Attr("max_connections_per_user", "MaxConnectionsPerUser", "Largest number of connections of one user.",
				int64validator.Between(1, 2147483647)),
			int64Attr("max_connection_from_single_ip", "MaxConnectionFromSingleIP",
				"Largest number of connections from one IP address.", int64validator.Between(1, 2147483647)),
		},
	}
}

// protocolLogNote: the transport services have no switch for the SMTP
// protocol logs, only their location and limits.
const protocolLogNote = "SMTP protocol logging is turned on for each connector, with its `protocol_logging_level`."

const maxAgeDesc = "Days after which log files are removed. `0` means no age limit."

// transportLogFields are the connectivity and SMTP protocol log settings that
// the Transport and Front End Transport services share. intraOrgDefault is the
// Exchange default of the intra-organization connector logging.
func transportLogFields(intraOrgDefault string) []field {
	fields := []field{
		boolAttr("connectivity_log_enabled", "ConnectivityLogEnabled", "Record connectivity logs (outgoing connections)."),
		pathAttr("connectivity_log_path", "ConnectivityLogPath", "Folder of the connectivity logs."),
		logAgeAttr("connectivity_log_max_age_days", "ConnectivityLogMaxAge", maxAgeDesc),
		sizeAttr("connectivity_log_max_directory_size_mb", "ConnectivityLogMaxDirectorySize",
			"Largest size, in megabytes, of the connectivity log folder. `0` means unlimited."),
		sizeAttr("connectivity_log_max_file_size_mb", "ConnectivityLogMaxFileSize",
			"Largest size, in megabytes, of a connectivity log file. `0` means unlimited."),
	}
	for _, dir := range []struct{ prefix, param, label string }{
		{"send", "SendProtocolLog", "Send"},
		{"receive", "ReceiveProtocolLog", "Receive"},
	} {
		fields = append(fields,
			pathAttr(dir.prefix+"_protocol_log_path", dir.param+"Path", "Folder of the protocol logs of the "+dir.label+" connectors."),
			logAgeAttr(dir.prefix+"_protocol_log_max_age_days", dir.param+"MaxAge", maxAgeDesc),
			sizeAttr(dir.prefix+"_protocol_log_max_directory_size_mb", dir.param+"MaxDirectorySize",
				"Largest size, in megabytes, of the folder of the protocol logs of the "+dir.label+" connectors. `0` means unlimited."),
			sizeAttr(dir.prefix+"_protocol_log_max_file_size_mb", dir.param+"MaxFileSize",
				"Largest size, in megabytes, of a protocol log file of the "+dir.label+" connectors. `0` means unlimited."),
		)
	}
	return append(fields, enumAttr("intra_org_connector_protocol_logging_level", "IntraOrgConnectorProtocolLoggingLevel",
		"Protocol logging of the connector that sends mail inside the organization, written to the folder of the Send "+
			"connectors. `"+intraOrgDefault+"` by default.", "None", "Verbose"))
}

// edgeNote ends the description of the anti-spam settings of Edge Transport servers.
const edgeNote = "An Edge Transport setting: on a Mailbox server, it needs the anti-spam agents and the Transport Hygiene role."

// ipListEntrySpec describes an entry of the IP allow or block list. An entry
// has no name: its IP address or range is its key, and there is no Set-
// cmdlet, so any change creates a new entry.
func ipListEntrySpec(typeName, noun, list, effect string) objectSpec {
	return objectSpec{
		typeName: typeName,
		noun:     noun,
		script:   "ip_list_entry.ps1",
		edgeOnly: true,
		listBy:   "IPRange",
		description: fmt.Sprintf("Manages an entry of the IP %s list of the connection filter agent: connections from the "+
			"address or range are %s", list, effect),
		notes: "`name` is the IP address (`192.0.2.10`), CIDR range (`192.0.2.0/24`) or address range " +
			"(`192.0.2.1-192.0.2.9`), as `Get-" + noun + "` shows it. Any change creates a new entry. " + edgeNote,
		fields: []field{
			{name: "comment", param: "Comment", kind: stringField, forceNew: true, desc: "Free text about the entry."},
			{name: "expiration_time", param: "ExpirationTime", kind: stringField, forceNew: true,
				desc: "When the entry expires, in RFC 3339 format and UTC, for example `2027-01-31T00:00:00Z`. Empty never expires."},
		},
	}
}

// virtualDirectorySpec describes the virtual directory of noun on the Default
// Web Site of a server, named "<dir> (Default Web Site)": its URLs, stored in
// Active Directory, and the authentication settings auth, stored in IIS. The
// script always reads IIS, so a read fails when the server's IIS does not
// answer rather than show AD-only placeholders.
func virtualDirectorySpec(typeName, noun, label, dir string, auth ...field) objectSpec {
	return objectSpec{
		typeName:    typeName,
		noun:        noun,
		script:      "virtual_directory.ps1",
		adopted:     true,
		description: fmt.Sprintf("Manages the URLs and the authentication settings of the %s virtual directory of an Exchange server", label),
		notes: fmt.Sprintf("`name` is the server name or FQDN, for the virtual directory of the Default Web Site, or its identity "+
			"as `Get-%s` shows it, such as `EX01\\%s (Default Web Site)`. "+
			"Authentication settings are read from IIS: a plan fails when the server's IIS does not answer.", noun, dir),
		fields: append([]field{
			stringAttr("internal_url", "InternalUrl", "URL used inside the network. An empty string clears it."),
			stringAttr("external_url", "ExternalUrl", "URL used from the internet. An empty string clears it."),
		}, auth...),
	}
}

// Authentication settings shared by several virtual directories.
var (
	basicAuthField   = boolAttr("basic_authentication", "BasicAuthentication", "Basic authentication in IIS.")
	windowsAuthField = boolAttr("windows_authentication", "WindowsAuthentication",
		"Integrated Windows authentication in IIS.")
	digestAuthField = boolAttr("digest_authentication", "DigestAuthentication", "Digest authentication in IIS.")
	oauthField      = boolAttr("oauth_authentication", "OAuthAuthentication", "OAuth authentication.")

	extendedProtectionField = readOnlyAttr(stringAttr("extended_protection_token_checking", "ExtendedProtectionTokenChecking",
		"Extended Protection: `None`, `Allow` or `Require`. Read only: set it with Microsoft's Extended Protection script."))
)

// formsAuthFields are the authentication settings of OWA and ECP. Exchange
// changes Basic, Windows, Digest and Forms together (turning Forms on sets
// Basic and turns Digest and Windows off): they are coupled.
func formsAuthFields() []field {
	return []field{coupledAttr("forms", basicAuthField), coupledAttr("forms", windowsAuthField),
		coupledAttr("forms", digestAuthField),
		coupledAttr("forms", boolAttr("forms_authentication", "FormsAuthentication", "Forms-based authentication.")),
		oauthField,
		boolAttr("adfs_authentication", "AdfsAuthentication", "AD FS claims-based authentication."),
	}
}

// withMethods adds the read-only settings of a directory that Autodiscover
// describes: Extended Protection and the methods Exchange advertises, which
// it derives from the IIS settings.
func withMethods(auth []field) []field {
	return append(auth, extendedProtectionField,
		readOnlyAttr(setAttr("internal_authentication_methods", "InternalAuthenticationMethods",
			"Authentication methods Exchange advertises to clients inside the network.")),
		readOnlyAttr(setAttr("external_authentication_methods", "ExternalAuthenticationMethods",
			"Authentication methods Exchange advertises to clients on the internet.")),
	)
}
