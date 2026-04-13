# "Load Balancer/Rate Limiter" на Golang разработчика

## Выбранные подходы
- [load balancer algorithm: Round Robin](#RoundRobin)
- [rate limiter algorithm: Token Bucket](#TokenBucket)


## Учтены все требования и рекомендации
-Проект разделен на модули (папка modules)

-Инициализация всех зависимостей и модулей происходит в app.go

-Добавлен ассинхронный логгер

-Работа RateLimiter'a реализована через каналы+мьютексы

-Конфигурация находится в файле config.yaml

-Посчитал, что будет производительнее и лаконичнее хранить данные во внутренних структурах, не в бд.

!Входные данные можно посылать как при помощи config файла, так и при помощи http запросов

(Ниже представлены их примеры)

Для загрузки проекта:
```sh
git clone https://github.com/MentalMentos/LoadBalancer_RateLimiter.git
```
Для запуска проекта через Docker:
```sh
docker-compose up -d
```
Для остановки:
```sh
docker-compose down
```

Для запуска проекта(без докера):
```golang
cd LoadBalancer_RateLimiter
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

#Тестирование apache bench
```sh
ab -n 100000 -c 100 -t 30 http://localhost:8080/clients
```
