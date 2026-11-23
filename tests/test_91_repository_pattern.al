// Copyright 2026 Pawl Programming Language
// Test 91: Repository pattern - data access abstraction.

// 1. Simple repository
struct UserRepository {
    users: map[int, string] = {}
}
func add_user(repo: UserRepository, id: int, name: string) -> UserRepository {
    repo.users[id] = name
    return repo
}
func get_user(repo: UserRepository, id: int) -> string {
    if id in repo.users {
        return repo.users[id]
    }
    return ""
}
repo := UserRepository()
repo = add_user(repo, 1, "Alice")
assert get_user(repo, 1) == "Alice"

// 2. Repository with update
func update_user(repo: UserRepository, id: int, name: string) -> UserRepository {
    if id in repo.users {
        repo.users[id] = name
    }
    return repo
}
repo = update_user(repo, 1, "Bob")
assert get_user(repo, 1) == "Bob"

// 3. Repository with delete
func delete_user(repo: UserRepository, id: int) -> UserRepository {
    if id in repo.users {
        result: map[int, string] = {}
        for key in repo.users {
            if key != id {
                result[key] = repo.users[key]
            }
        }
        repo.users = result
    }
    return repo
}
repo = delete_user(repo, 1)
assert get_user(repo, 1) == ""

// 4. Repository count
func count_users(repo: UserRepository) -> int {
    count := 0
    for key in repo.users {
        count = count + 1
    }
    return count
}
repo2 := UserRepository()
repo2 = add_user(repo2, 1, "Alice")
repo2 = add_user(repo2, 2, "Bob")
assert count_users(repo2) == 2

// 5. Repository find all
func find_all_users(repo: UserRepository) -> list[string] {
    result: list[string] = []
    for key in repo.users {
        result = result + [repo.users[key]]
    }
    return result
}
all_users := find_all_users(repo2)
assert len(all_users) == 2

// 6. Repository exists check
func user_exists(repo: UserRepository, id: int) -> bool {
    return id in repo.users
}
assert user_exists(repo2, 1)
assert !user_exists(repo2, 3)

// 7. Repository clear
func clear_users(repo: UserRepository) -> UserRepository {
    repo.users = {}
    return repo
}
repo2 = clear_users(repo2)
assert count_users(repo2) == 0

// 8. Repository with filter
func find_users_by_name(repo: UserRepository, name: string) -> list[int] {
    result: list[int] = []
    for key in repo.users {
        if repo.users[key] == name {
            result = result + [key]
        }
    }
    return result
}
repo3 := UserRepository()
repo3 = add_user(repo3, 1, "Alice")
repo3 = add_user(repo3, 2, "Alice")
found := find_users_by_name(repo3, "Alice")
assert len(found) == 2

// 9. Repository batch operation
func add_users_batch(repo: UserRepository, users: map[int, string) -> UserRepository:
    for id in users {
        repo.users[id] = users[id]
    }
    return repo

batch := {3: "Charlie", 4: "David"}
repo3 = add_users_batch(repo3, batch)
assert count_users(repo3) == 4

// 10. Repository transaction simulation
func transaction(repo: UserRepository, operations: func(UserRepository) -> UserRepository) -> UserRepository {
    return operations(repo)
}
transacted := transaction(repo3, func(r: UserRepository) -> UserRepository:
    r = add_user(r, 5, "Eve")
    return r
)
assert get_user(transacted, 5) == "Eve"

println("✓ test_91_repository_pattern passed")
