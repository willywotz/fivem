package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

func main() {
	// if !checkAdmin() {
	// 	_ = becomeAdmin()
	// 	return
	// }

	// time.Sleep(2 * time.Second) // Simulate some startup delay
	// PressAndRelease(VK_E)

	myApp := app.New()
	myWindow := myApp.NewWindow("List Data")

	data := binding.BindStringList(
		&[]string{"Item 1", "Item 2", "Item 3"},
	)

	list := widget.NewListWithData(data,
		func() fyne.CanvasObject {
			return widget.NewLabel("template")
		},
		func(i binding.DataItem, o fyne.CanvasObject) {
			o.(*widget.Label).Bind(i.(binding.String))
		})

	add := widget.NewButton("Append", func() {
		val := fmt.Sprintf("Item %d", data.Length()+1)
		data.Append(val)
	})
	myWindow.SetContent(container.NewBorder(nil, add, nil, nil, list))
	myWindow.ShowAndRun()
}

// func main() {
// 	myApp := app.New()
// 	myWindow := myApp.NewWindow("My Fyne App")

// 	// Set the content of your window
// 	myWindow.SetContent(container.NewVBox(
// 		widget.NewLabel("Hello Fyne!"),
// 		widget.NewButton("Click me", func() {
// 			log.Println("Button clicked!")
// 		}),
// 	))

// 	// 1. Intercept the close button click
// 	myWindow.SetCloseIntercept(func() {
// 		log.Println("Close button clicked, hiding window...")
// 		myWindow.Hide() // Hide the window instead of closing it
// 	})

// 	// 2. Set up the system tray menu (desktop specific)
// 	if desk, ok := myApp.(desktop.App); ok {
// 		// Create a menu for the system tray
// 		m := fyne.NewMenu("My Fyne App",
// 			fyne.NewMenuItem("Show", func() {
// 				myWindow.Show() // Show the window again
// 			}),
// 			fyne.NewMenuItem("Quit", func() {
// 				myApp.Quit() // Explicitly quit the application
// 			}),
// 		)
// 		desk.SetSystemTrayMenu(m)

// 		// Optionally, set a custom icon for the system tray
// 		// desk.SetSystemTrayIcon(resource.MyCustomIcon) // Replace with your icon
// 	}

// 	myWindow.ShowAndRun()
// }

// func main() {
// 	// if err := setDefaultMicrophoneVolume(); err != nil {
// 	// 	fmt.Printf("Error setting default microphone volume: %v\n", err)
// 	// }

// 	// _, err := getAudioDeviceCollection()
// 	// if err != nil {
// 	// 	fmt.Printf("Error getting audio device collection: %v\n", err)
// 	// 	return
// 	// }
// }

// func main() {
// 	if !checkAdmin() {
// 		fmt.Println("Not running as administrator. Attempting to elevate privileges...")

// 		if err := becomeAdmin(); err != nil {
// 			fmt.Printf("Failed to elevate privileges: %v\n", err)
// 			fmt.Println("Please run this application as an administrator.")
// 			return
// 		}

// 		// It's crucial to exit the current non-admin process after attempting elevation.
// 		// The new elevated process will continue the execution.
// 		os.Exit(0)
// 	}

// 	fmt.Println("Running as administrator! Performing administrative tasks...")
// 	// Your administrative code here
// 	// Example: Create a file in a protected directory
// 	file, err := os.Create("C:\\Program Files\\MyAdminApp\\testfile.txt")
// 	if err != nil {
// 		fmt.Printf("Error creating file: %v\n", err)
// 	} else {
// 		fmt.Println("Successfully created testfile.txt in Program Files.")
// 		file.Close()
// 	}

// 	fmt.Println("Press Enter to exit.")
// 	fmt.Scanln()
// }

// 	a := app.New()

// 	// Check if running on a desktop environment to use desktop-specific features
// 	if desk, ok := a.Driver().(desktop.Driver); ok {
// 		// Create a splash window (borderless)
// 		splashWindow := desk.CreateSplashWindow()
// 		splashWindow.Resize(fyne.NewSize(400, 200))

// 		splashWindow.SetContent(widget.NewLabel("Loading..."))
// 		splashWindow.CenterOnScreen()
// 		splashWindow.ShowAndRun()

// 		// Simulate some loading time
// 		// time.Sleep(3 * time.Second)

// 		// Close the splash window and open your main application window
// 		// splashWindow.Close()

// 		// mainWindow := a.NewWindow("My Application")
// 		// mainWindow.SetContent(container.NewVBox(
// 		// 	widget.NewLabel("Welcome to my Fyne app! a"),
// 		// 	widget.NewButton("Click Me", func() {
// 		// 		// Do something
// 		// 	}),
// 		// ))
// 		// mainWindow.ShowAndRun()

// 	} else {
// 		// Fallback for non-desktop environments or if desktop features are not available
// 		w := a.NewWindow("My Application")
// 		w.SetContent(container.NewVBox(
// 			widget.NewLabel("Welcome to my Fyne app! b"),
// 			widget.NewButton("Click Me", func() {
// 				// Do something
// 			}),
// 		))
// 		w.ShowAndRun()
// 	}
// }
