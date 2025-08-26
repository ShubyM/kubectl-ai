# Mocking in Kubectl ai

## Gomock developer workflow
We use [gomock](https://github.com/uber-go/mock) to mock external dependencies. All of our mocks and generated files are stored under internal/mocks/ directory. 

If you modify an interface that has been mocked already or want to mock something new you can run `make generate`. If you aren't sure you can run
`make verify-mocks` to be sure that all mocks are up to date. 
