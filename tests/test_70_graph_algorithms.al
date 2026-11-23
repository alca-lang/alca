// Copyright 2026 Pawl Programming Language
// Test 70: Graph algorithms - BFS, DFS simulation.

// 1. Graph node structure
struct GraphNode {
    id: int
    neighbors: list[int] = []

// 2. Create graph
}
func create_graph() -> map[int, GraphNode] {
    graph: map[int, GraphNode] = {}
    graph[0] = GraphNode(0, [1, 2])
    graph[1] = GraphNode(1, [0, 2, 3])
    graph[2] = GraphNode(2, [0, 1, 3])
    graph[3] = GraphNode(3, [1, 2])
    return graph
}
graph := create_graph()
assert len(graph) == 4

// 3. BFS simulation
func bfs(graph: map[int, GraphNode], start: int) -> list[int] {
    visited: list[int] = []
    queue: list[int] = [start]
    while len(queue) > 0 {
        node := queue[0]
        queue = queue[1..]
        if !(node in visited) {
            visited = visited + [node]
            if node in graph {
                for neighbor in graph[node].neighbors {
                    if !(neighbor in visited) {
                        queue = queue + [neighbor]
                    }
                }
            }
        }
    }
    return visited
}
bfs_result := bfs(graph, 0)
assert len(bfs_result) == 4

// 4. DFS simulation
func dfs(graph: map[int, GraphNode], start: int) -> list[int] {
    visited: list[int] = []
    stack: list[int] = [start]
    while len(stack) > 0 {
        node := stack[len(stack) - 1]
        stack = stack[0..len(stack) - 1]
        if !(node in visited) {
            visited = visited + [node]
            if node in graph {
                i := len(graph[node].neighbors) - 1
                while i >= 0 {
                    neighbor := graph[node].neighbors[i]
                    if !(neighbor in visited) {
                        stack = stack + [neighbor]
                    }
                    i = i - 1
                }
            }
        }
    }
    return visited
}
dfs_result := dfs(graph, 0)
assert len(dfs_result) == 4

// 5. Check if path exists
func has_path(graph: map[int, GraphNode, start: int, end: int) -> bool:
    if start == end {
        return true
    }
    visited: list[int] = []
    queue: list[int] = [start]
    while len(queue) > 0 {
        node := queue[0]
        queue = queue[1..]
        if node == end {
            return true
        }
        if !(node in visited) {
            visited = visited + [node]
            if node in graph {
                for neighbor in graph[node].neighbors {
                    if !(neighbor in visited) {
                        queue = queue + [neighbor]
                    }
                }
            }
        }
    }
    return false

assert has_path(graph, 0, 3)

// 6. Get neighbors
func get_neighbors(graph: map[int, GraphNode, node: int) -> list[int]:
    if node in graph {
        return graph[node].neighbors
    }
    return []

neighbors := get_neighbors(graph, 0)
assert neighbors == [1, 2]

// 7. Count nodes
func count_nodes(graph: map[int, GraphNode) -> int:
    count := 0
    for key in graph {
        count = count + 1
    }
    return count

assert count_nodes(graph) == 4

// 8. Count edges
func count_edges(graph: map[int, GraphNode) -> int:
    count := 0
    for key in graph {
        count = count + len(graph[key].neighbors)
    }
    return count / 2 // Undirected graph

assert count_edges(graph) == 4

// 9. Add edge
func add_edge(graph: map[int, GraphNode, from: int, to: int) -> map[int, GraphNode]:
    if from in graph {
        graph[from].neighbors = graph[from].neighbors + [to]
    }
    if to in graph {
        graph[to].neighbors = graph[to].neighbors + [from]
    }
    return graph

graph2 := create_graph()
graph2 = add_edge(graph2, 0, 3)
assert 3 in graph2[0].neighbors

// 10. Remove edge simulation
func remove_edge(graph: map[int, GraphNode, from: int, to: int) -> map[int, GraphNode]:
    if from in graph {
        new_neighbors: list[int] = []
        for neighbor in graph[from].neighbors {
            if neighbor != to {
                new_neighbors = new_neighbors + [neighbor]
            }
        }
        graph[from].neighbors = new_neighbors
    }
    return graph

graph3 := create_graph()
graph3 = remove_edge(graph3, 0, 1)
assert !(1 in graph3[0].neighbors)

println("✓ test_70_graph_algorithms passed")
