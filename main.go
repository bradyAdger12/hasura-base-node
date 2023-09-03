package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"

	"aaronblondeau.com/hasura-base-go/controllers"
	"github.com/aaronblondeau/crew-go/crew"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	e := echo.New()

	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "hasura-base-go!")
	})

	e.GET("/readycheck", func(c echo.Context) error {
		return c.String(http.StatusOK, "I'm ready!")
	})

	e.GET("/healthcheck", func(c echo.Context) error {
		hasuraBaseUrl := os.Getenv("HASURA_BASE_URL")
		if hasuraBaseUrl == "" {
			hasuraBaseUrl = "http://localhost:8080"
		}
		url := hasuraBaseUrl + "/v1/version"
		resp, requestErr := http.Get(url)
		if requestErr != nil {
			return c.String(http.StatusInternalServerError, "Failed to fetch hasura status!")
		}
		defer resp.Body.Close()

		var response interface{}
		jsonErr := json.NewDecoder(resp.Body).Decode(&response)
		if jsonErr != nil {
			return c.String(http.StatusInternalServerError, "Failed to parse hasura status!")
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"healthy": true,
			"hasura":  response,
		})
	})

	actionsController := controllers.NewActionsController(e)
	if err := actionsController.Run(e); err != nil {
		panic(err)
	}

	authController := controllers.NewAuthController(e)
	if err := authController.Run(e); err != nil {
		panic(err)
	}

	host := os.Getenv("HOST")
	if host == "" {
		// use localhost on windows, 0.0.0.0 elsewhere
		if runtime.GOOS == "windows" {
			host = "localhost"
		} else {
			host = "0.0.0.0"
		}
	}

	//////////////// Start Crew Setup

	crewRedisAddress := os.Getenv("REDIS_ADDRESS")
	if crewRedisAddress == "" {
		crewRedisAddress = "localhost:6379"
	}
	crewRedisPassword := os.Getenv("REDIS_PASSWORD")

	storage := crew.NewRedisTaskStorage(crewRedisAddress, crewRedisPassword, 1)
	defer storage.Client.Close()

	crewClient := crew.NewHttpPostClient()

	throttlePush := make(chan crew.ThrottlePushQuery, 8)
	throttlePop := make(chan crew.ThrottlePopQuery, 8)
	crewThrottler := &crew.Throttler{
		Push: throttlePush,
		Pop:  throttlePop,
	}

	// No throttling for crew
	go func() {
		for {
			select {
			case pushQuery := <-throttlePush:
				// Default behavior = immediate response => no throttling
				fmt.Println("~~ Would throttle", pushQuery.Worker, pushQuery.TaskId)
				pushQuery.Resp <- true
			case popQuery := <-throttlePop:
				fmt.Println("~~ Would unthrottle", popQuery.Worker, popQuery.TaskId)
			}
		}
	}()

	// Create the task controller (call to startup is further down)
	crewController := crew.NewTaskController(storage, crewClient, crewThrottler)

	// Validates each api call's Authorization header
	crewAuthMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// For systems requiring no auth, just return next(c)
			return next(c)
		}
	}

	// Create the rest api server
	inShutdown := false
	watchers := make(map[string]crew.TaskGroupWatcher, 0)
	crew.BuildRestApi(e, "/crew", crewController, crewAuthMiddleware, nil, &inShutdown, watchers)

	// TODO - add worker routes to crewEcho

	// Controller startup is performed after rest api is launched
	// This is in case we switch TaskController.TriggerEvaluate to happen via an http call in scaled environments.
	startupError := crewController.Startup()
	if startupError != nil {
		panic(startupError)
	}

	//////////////// End Crew Setup

	// Hook into the shutdown signal
	cleanupCompleteWg := sync.WaitGroup{}
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-done
		// sigint caught, start graceful shutdown
		log.Print("Process Terminating...")

		// Call shutdown on each controller
		actionsController.Shutdown()
		authController.Shutdown()

		// Shutdown crew
		for _, watcher := range watchers {
			close(watcher.Channel)
		}

		// Shutdown echo
		e.Close()

		cleanupCompleteWg.Done()
	}()
	cleanupCompleteWg.Add(1)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	e.Logger.Error(e.Start(host + ":" + port))

	cleanupCompleteWg.Wait()
}
