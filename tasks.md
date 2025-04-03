## Tasks

- [] Develop the shrink startegy

## Info

- Background thread will check some stats and make a decision if we should shrink the pool or not.
- The trigger for shrink will be a combination of factors that come from the pool stats.

## MINI-TASKS

- [x] develop the pool stats that will be useful
- [x] develop **PoolShrinkParameters** based on what will be needed to trigger a shrink.
- [x] default levels config
- [x] need to have more control over the pool creation
- [x] change pool usage to allow only pointers to be stored.
- [] help users create the config ?
  - [] I want users to use a builder to build the config that will be passed to whatever needs it.
  - [] many structs and types shouldn't be accessible outside of the library.
- [] when and how do we collect the stats

## Help Users

- Need to implement rules such as:
  - if EnableAutoShrink is true, AggressivenessLevel can't be 0
  - if EnableAutoShrink is false, AggressivenessLevel can't be different than 0
  - if EnableAutoShrink is false, then no config value can be default.
  - make sure the parameters chosen aren't insane, or that they aren't incorrect
