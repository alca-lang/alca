// Copyright 2026 Pawl Programming Language
// Test 71: Tree traversal - inorder, preorder, postorder.

// 1. Tree node structure
struct TreeNode {
    value: int
    left: TreeNode = null
    right: TreeNode = null

// 2. Create binary tree
}
func create_tree() -> TreeNode {
    root := TreeNode(10, null, null)
    root.left = TreeNode(5, null, null)
    root.right = TreeNode(15, null, null)
    root.left.left = TreeNode(2, null, null)
    root.left.right = TreeNode(7, null, null)
    root.right.left = TreeNode(12, null, null)
    root.right.right = TreeNode(20, null, null)
    return root
}
tree := create_tree()
assert tree.value == 10

// 3. Inorder traversal
func inorder(node: TreeNode) -> list[int] {
    result: list[int] = []
    if node != null {
        left_result := inorder(node.left)
        result = result + left_result
        result = result + [node.value]
        right_result := inorder(node.right)
        result = result + right_result
    }
    return result
}
inorder_result := inorder(tree)
assert inorder_result == [2, 5, 7, 10, 12, 15, 20]

// 4. Preorder traversal
func preorder(node: TreeNode) -> list[int] {
    result: list[int] = []
    if node != null {
        result = result + [node.value]
        left_result := preorder(node.left)
        result = result + left_result
        right_result := preorder(node.right)
        result = result + right_result
    }
    return result
}
preorder_result := preorder(tree)
assert preorder_result == [10, 5, 2, 7, 15, 12, 20]

// 5. Postorder traversal
func postorder(node: TreeNode) -> list[int] {
    result: list[int] = []
    if node != null {
        left_result := postorder(node.left)
        result = result + left_result
        right_result := postorder(node.right)
        result = result + right_result
        result = result + [node.value]
    }
    return result
}
postorder_result := postorder(tree)
assert postorder_result == [2, 7, 5, 12, 20, 15, 10]

// 6. Level order (BFS)
func level_order(root: TreeNode) -> list[int] {
    result: list[int] = []
    if root == null {
        return result
    }
    queue: list[TreeNode] = [root]
    while len(queue) > 0 {
        node := queue[0]
        queue = queue[1..]
        result = result + [node.value]
        if node.left != null {
            queue = queue + [node.left]
        }
        if node.right != null {
            queue = queue + [node.right]
        }
    }
    return result
}
level_result := level_order(tree)
assert level_result == [10, 5, 15, 2, 7, 12, 20]

// 7. Tree height
func tree_height(node: TreeNode) -> int {
    if node == null {
        return 0
    }
    left_height := tree_height(node.left)
    right_height := tree_height(node.right)
    if left_height > right_height {
        return left_height + 1
    }
    return right_height + 1
}
height := tree_height(tree)
assert height == 3

// 8. Count nodes
func count_nodes(node: TreeNode) -> int {
    if node == null {
        return 0
    }
    return 1 + count_nodes(node.left) + count_nodes(node.right)
}
node_count := count_nodes(tree)
assert node_count == 7

// 9. Count leaves
func count_leaves(node: TreeNode) -> int {
    if node == null {
        return 0
    }
    if node.left == null and node.right == null {
        return 1
    }
    return count_leaves(node.left) + count_leaves(node.right)
}
leaf_count := count_leaves(tree)
assert leaf_count == 4

// 10. Find value in tree
func find_value(node: TreeNode, target: int) -> bool {
    if node == null {
        return false
    }
    if node.value == target {
        return true
    }
    if target < node.value {
        return find_value(node.left, target)
    }
    return find_value(node.right, target)
}
assert find_value(tree, 7)
assert !find_value(tree, 100)

// 11. Insert into BST
func insert_bst(node: TreeNode, value: int) -> TreeNode {
    if node == null {
        return TreeNode(value, null, null)
    }
    if value < node.value {
        node.left = insert_bst(node.left, value)
    }
    else {
        node.right = insert_bst(node.right, value)
    }
    return node
}
bst := TreeNode(10, null, null)
bst = insert_bst(bst, 5)
bst = insert_bst(bst, 15)
assert bst.left.value == 5
assert bst.right.value == 15

// 12. Find min value
func find_min(node: TreeNode) -> int {
    if node == null {
        return 0
    }
    if node.left == null {
        return node.value
    }
    return find_min(node.left)
}
min_val := find_min(tree)
assert min_val == 2

// 13. Find max value
func find_max(node: TreeNode) -> int {
    if node == null {
        return 0
    }
    if node.right == null {
        return node.value
    }
    return find_max(node.right)
}
max_val := find_max(tree)
assert max_val == 20

// 14. Check if BST
func is_bst(node: TreeNode, min: int, max: int) -> bool {
    if node == null {
        return true
    }
    if node.value < min or node.value > max {
        return false
    }
    return is_bst(node.left, min, node.value - 1) and
           is_bst(node.right, node.value + 1, max)
}
assert is_bst(tree, -1000, 1000)

// 15. Tree diameter (longest path)
func tree_diameter(node: TreeNode) -> int {
    if node == null {
        return 0
    }
    left_height := tree_height(node.left)
    right_height := tree_height(node.right)
    left_diameter := tree_diameter(node.left)
    right_diameter := tree_diameter(node.right)
    max_child_diameter := left_diameter
    if right_diameter > max_child_diameter {
        max_child_diameter = right_diameter
    }
    return max_child_diameter + 1
}
diameter := tree_diameter(tree)
assert diameter > 0

println("✓ test_71_tree_traversal passed")
