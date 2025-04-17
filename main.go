package main

import (
	"memctx/contexts"
)

type Example struct {
	ID int
}

func main() {
	cm := contexts.NewContextManager(nil)

	ctx, _, err := contexts.GetOrCreateTypedPool[*Example](cm, "test", contexts.MemoryContextConfig{
		Parent:      nil,
		ContextType: "test",
	})

	if err != nil {
		panic(err)
	}

	err = ctx.CreatePool(nil,
		func() *Example { return &Example{} },
		func(obj *Example) { obj.ID = 0 },
	)

	if err != nil {
		panic(err)
	}

	example := ctx.Acquire()
	example.ID = 1

	ctx.Release(example)
}
