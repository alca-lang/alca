use std.ai;
func bad():
    println(ai.matmul([[1]], [[1, 2]]));
    println(ai.matmul([[1, 2]], [[1, 2]]));
bad();