provider "aws" {
  version = "~> 1.52.0"
  alias   = "default"

  region                  = "us-east-1"
  profile = "riot"
}

provider "aws" {
  version = "~> 1.52.0"
  alias   = "utap"

  region = "us-east-1"
  profile = "default"
}

provider "ct" {
  version = "0.3.0"
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