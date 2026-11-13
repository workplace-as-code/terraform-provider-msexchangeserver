# Data sources built from the resources: each one reads the test object of its
# resource by name and must find the same object.

data "msexchangeserver_remote_domain" "test" {
  name       = msexchangeserver_remote_domain.test.name
  depends_on = [msexchangeserver_remote_domain.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeserver_remote_domain.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeserver_transport_rule" "test" {
  name       = msexchangeserver_transport_rule.test.name
  depends_on = [msexchangeserver_transport_rule.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeserver_transport_rule.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeserver_owa_mailbox_policy" "test" {
  name       = msexchangeserver_owa_mailbox_policy.test.name
  depends_on = [msexchangeserver_owa_mailbox_policy.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeserver_owa_mailbox_policy.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeserver_mobile_device_mailbox_policy" "test" {
  name       = msexchangeserver_mobile_device_mailbox_policy.test.name
  depends_on = [msexchangeserver_mobile_device_mailbox_policy.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeserver_mobile_device_mailbox_policy.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeserver_email_address_policy" "test" {
  name       = msexchangeserver_email_address_policy.test.name
  depends_on = [msexchangeserver_email_address_policy.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeserver_email_address_policy.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeserver_retention_policy_tag" "test" {
  name       = msexchangeserver_retention_policy_tag.test.name
  depends_on = [msexchangeserver_retention_policy_tag.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeserver_retention_policy_tag.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeserver_retention_policy" "test" {
  name       = msexchangeserver_retention_policy.test.name
  depends_on = [msexchangeserver_retention_policy.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeserver_retention_policy.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeserver_exchange_certificate" "test" {
  server     = msexchangeserver_exchange_certificate.test.server
  thumbprint = msexchangeserver_exchange_certificate.test.thumbprint
  depends_on = [msexchangeserver_exchange_certificate_services.test]

  lifecycle {
    postcondition {
      condition     = self.subject == msexchangeserver_exchange_certificate.test.subject && contains(self.services, "POP")
      error_message = "The data source read another certificate than the test one, or without its POP service."
    }
    postcondition {
      condition     = self.default_smtp_certificate == local.updated
      error_message = "The test certificate is the default SMTP certificate only when default_smtp_certificate is true."
    }
  }
}

# The data sources read IIS like the resources: they must show the same
# authentication settings, not the False that Active Directory alone gives.
data "msexchangeserver_oab_virtual_directory" "test" {
  name       = msexchangeserver_oab_virtual_directory.test.name
  depends_on = [msexchangeserver_oab_virtual_directory.test]

  lifecycle {
    postcondition {
      condition     = self.basic_authentication == msexchangeserver_oab_virtual_directory.test.basic_authentication
      error_message = "The data source shows another basic_authentication than the resource."
    }
  }
}

data "msexchangeserver_outlook_anywhere" "test" {
  name       = msexchangeserver_outlook_anywhere.test.name
  depends_on = [msexchangeserver_outlook_anywhere.test]

  lifecycle {
    postcondition {
      condition     = length(self.iis_authentication_methods) > 0
      error_message = "The data source shows no IIS authentication method: IIS was not read."
    }
  }
}

data "msexchangeserver_transport_service" "test" {
  name       = msexchangeserver_transport_service.test.name
  depends_on = [msexchangeserver_transport_service.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeserver_transport_service.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeserver_frontend_transport_service" "test" {
  name       = msexchangeserver_frontend_transport_service.test.name
  depends_on = [msexchangeserver_frontend_transport_service.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeserver_frontend_transport_service.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeserver_pop_settings" "test" {
  name       = msexchangeserver_pop_settings.test.name
  depends_on = [msexchangeserver_pop_settings.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeserver_pop_settings.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeserver_imap_settings" "test" {
  name       = msexchangeserver_imap_settings.test.name
  depends_on = [msexchangeserver_imap_settings.test]

  lifecycle {
    postcondition {
      condition     = self.id == msexchangeserver_imap_settings.test.id
      error_message = "The data source read another object than the test one."
    }
  }
}

data "msexchangeserver_address_book_policy" "test" {
  name       = msexchangeserver_address_book_policy.test.name
  depends_on = [msexchangeserver_address_book_policy.test]

  lifecycle {
    postcondition {
      condition     = self.global_address_list == "TF-CI-GAL" && self.room_list == "TF-CI-Rooms"
      error_message = "The data source did not read the references of the policy as names."
    }
  }
}

data "msexchangeserver_address_list" "child" {
  name       = msexchangeserver_address_list.child.name
  depends_on = [msexchangeserver_address_list.child]

  lifecycle {
    postcondition {
      condition     = self.container == "TF-CI-AL"
      error_message = "The data source did not read the container of the nested list."
    }
  }
}
