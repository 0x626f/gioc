package gioc

type state int

const (
	visiting state = iota
	visited
)

type nodeConfig[T Tokenized] struct {
	Neighbors func(T) ([]T, error)

	ShouldVisit func(T) bool

	OnVisited func(T) error

	OnCycle func([]Token) error
}

func dfs[T Tokenized](roots []T, cfg nodeConfig[T]) error {
	states := make(map[Token]state)

	var walk func(node T, path []string) error
	walk = func(node T, path []string) error {
		if s, exists := states[node.Token()]; exists {
			if s == visited {
				return nil
			}
			path = append(path, node.Token())

			if cfg.OnCycle != nil {
				return cfg.OnCycle(path)
			}
			return circularDependencyInjection(path...)
		}

		states[node.Token()] = visiting
		key := node.Token()
		path = append(path[:len(path):len(path)], key)

		neighbors, err := cfg.Neighbors(node)
		if err != nil {
			return err
		}

		for _, neighbor := range neighbors {
			if cfg.ShouldVisit != nil && !cfg.ShouldVisit(neighbor) {
				continue
			}
			if err := walk(neighbor, path); err != nil {
				return err
			}
		}

		if cfg.OnVisited != nil {
			if err := cfg.OnVisited(node); err != nil {
				return err
			}
		}

		states[node.Token()] = visited
		return nil
	}

	for _, root := range roots {
		if err := walk(root, nil); err != nil {
			return err
		}
	}

	return nil
}
