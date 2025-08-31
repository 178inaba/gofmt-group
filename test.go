package main

type MyInterface interface {
	Get(c echo.Context) (error)
	Update(id int, name string, age int, city string) (error)
	Process(a, b int, c, d string) (User, error)
}

func Get(c echo.Context) error {
	return nil
}

func Update(id int, name string, age int, city string) error {
	return nil
}
