package rebac

import "fmt"

func Example() {
	fmt.Println(UserSubject("user-1"))
	fmt.Println(RepositoryObject(42))
	// Output:
	// user:user-1
	// repository:42
}
