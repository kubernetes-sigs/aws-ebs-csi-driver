provider "aws" {
  version = "~> 1.13.0"
  alias   = "default"

  region                  = "us-east-1"
  shared_credentials_file = "/home/shane/.config/aws/credentials"
}

provider "ct" {
  version = "0.2.1"
}

provider "local" {
  version = "~> 1.0"
  alias = "default"
}

provider "null" {
  version = "~> 1.0"
  alias = "default"
}

provider "template" {
  version = "~> 1.0"
  alias = "default"
}

provider "tls" {
  version = "~> 1.0"
  alias = "default"
}