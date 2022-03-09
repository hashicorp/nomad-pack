app {
  url = ""
  author = "HashiCorp"
}

pack {
  name = "deps_test"
  description = "This pack tests dependencies"
  url = "github.com/hashicorp/nomad-pack/fixtures/test_registry/packs/deps_test"
  version = "0.0.1"
}

dependency "child1" {
  source = "./deps/child1"
}
dependency "child2" {
  source = "./deps/child2"
}
