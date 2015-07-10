load "fixtures/basics"
load "fixtures/org"

@test "org show" {
  init_init
  init
  run org_show
  [ "$status" -eq 0 ]
  cleanup
}

@test "out of org dir" {
  init_init
  init
  cd ..
  run org_show
  [ "$status" -eq 1 ]
  [[ "$output" =~ "Couldn't read org config" ]]
}
