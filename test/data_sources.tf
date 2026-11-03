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
