package payment

//func TestGenerateOrderNumber(t *testing.T) {
//	var wg sync.WaitGroup
//	for i := 0; i < 10; i++ {
//		wg.Add(1)
//		go func() {
//			defer wg.Done()
//			n, err := GenerateOrderNumber()
//		}()
//	}
//	wg.Wait()
//}
//
//func TestGenerateOrderNumberBySnowFlake(t *testing.T) {
//	var wg sync.WaitGroup
//	for i := 0; i < 10; i++ {
//		wg.Add(1)
//		go func() {
//			defer wg.Done()
//			fmt.Println(GenerateOrderNumberBySnowFlake(1))
//		}()
//	}
//	wg.Wait()
//}
//
