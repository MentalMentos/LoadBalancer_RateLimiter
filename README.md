# Тестовое задание для cloud ru "Load Balancer/Rate Limiter"


1. Вопросы для разогрева
Опишите самую интересную задачу в программировании, которую вам приходилось решать?

Расскажите о своем самом большом факапе? Что вы предприняли для решения проблемы?

Каковы ваши ожидания от участия в буткемпе?

# Проект
## Выбранные подходы
- [load balancer algorithm: Round Robin](#RoundRobin)
- [rate limiter algorithm: Token Bucket](#TokenBucket)


## Учтены все требования и рекомендации

Для запуска проекта:
```golang
go run cmd/app/main.go
```
### Установка зависимостей
Для установки зависимостей, выполните команду:
```sh
go mod tidy
```
#Пример post запроса
```sh
curl -X POST -H "Content-Type: application/json" -d '{
    "client_ip":"192.168.1.5",
    "capacity": 100,
    "interval": 10
}' http://localhost:8080/clients
```
!Если "client_ip" пуст -> сервис сам парсит ip

#Пример get запроса
```sh
curl -X GET http://localhost:8080/clients
```
