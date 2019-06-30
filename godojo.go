package main

// TODO:
// Add Cobra for command-line args - https://github.com/spf13/cobra
// Add redactatron function like prior installer
import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mtesauro/godojo/config"
	"github.com/mtesauro/godojo/util"
	"github.com/spf13/viper"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

// Global vars
var (
	// For logging
	logLocation = "logs"
	Trace       *log.Logger
	Info        *log.Logger
	Warning     *log.Logger
	Error       *log.Logger
	// For Global config flags
	Quiet   bool
	TraceOn bool
	// URLs needed by the installer
	HelpURL    = "https://github.com/mtesauro/godojo"
	ReleaseURL = "https://github.com/DefectDojo/django-DefectDojo/archive/"
	CloneURL   = "https://github.com/DefectDojo/django-DefectDojo.git"
)

// Setup logging with type appended to the log lines - this logs all types to a single file
func logSetup(logHandler io.Writer) {
	// Setup logging 'levels' which can be called globally like Info.Println("Example info log")
	Trace = log.New(logHandler, "TRACE:   ", log.Ldate|log.Ltime)
	Info = log.New(logHandler, "INFO:    ", log.Ldate|log.Ltime)
	Warning = log.New(logHandler, "WARNING: ", log.Ldate|log.Ltime)
	Error = log.New(logHandler, "ERROR:   ", log.Ldate|log.Ltime)
}

// Output the installer banner
func dojoBanner() {
	fmt.Println("        ____       ____          __     ____          _      ")
	fmt.Println("       / __ \\___  / __/__  _____/ /_   / __ \\____    (_)___  ")
	fmt.Println("      / / / / _ \\/ /_/ _ \\/ ___/ __/  / / / / __ \\  / / __ \\ ")
	fmt.Println("     / /_/ /  __/ __/  __/ /__/ /_   / /_/ / /_/ / / / /_/ / ")
	fmt.Println("    /_____/\\___/_/  \\___/\\___/\\__/  /_____/\\____/_/ /\\____/  ")
	fmt.Println("                                               /___/         ")
	fmt.Println("")
	fmt.Println("  Welcome to goDojo, the official way to install DefectDojo.")
	fmt.Println("  For more information on how goDojo does an install, see:")
	fmt.Print("  %s", HelpURL)
	fmt.Println("")
}

// Output a section message and log the same string
func sectionMsg(s string) {
	// Pring status message if quiet isn't set
	if !Quiet {
		fmt.Println("")
		fmt.Println("==============================================================================")
		fmt.Printf("  %s\n", s)
		fmt.Println("==============================================================================")
		fmt.Println("")
	}
	Info.Println("SECTION: " + s)
}

// Output a status message and log the same string
func statusMsg(s string) {
	// Pring status message if quiet isn't set
	if !Quiet {
		fmt.Printf("%s\n", s)
	}
	Info.Println(s)
}

// Output a blatant error message and log the string as an error
func errorMsg(s string) {
	// Pring status message if quiet isn't set
	if !Quiet {
		fmt.Println("")
		fmt.Println("##############################################################################")
		fmt.Printf("  ERROR: %s\n", s)
		fmt.Println("##############################################################################")
		fmt.Println("")
	}
	Error.Println(s)
}

// Output a blatant error message and log the string as an error
func traceMsg(s string) {
	// Pring status message if quiet isn't set
	if TraceOn {
		Trace.Println(s)
	}
}

// getDojo retrives the supplied version of DefectDojo from the Git repo
// and places it in the specified dojoSource directory (default is /opt/dojo)
func getDojoRelease(i *config.InstallConfig) error {
	statusMsg("Downloading the configured release of DefectDojo")

	// Setup needed info
	dwnURL := ReleaseURL + i.Version + ".tar.gz"
	tarball := i.Root + "/dojo-v" + i.Version + ".tar.gz"
	traceMsg(fmt.Sprintf("Relese download list is %+v", dwnURL))
	traceMsg(fmt.Sprintf("File path to write tarball is %+v", tarball))

	// Setup a custom http client for downloading the Dojo release
	var ddClient = &http.Client{
		// Set time to a max of 20 seconds
		Timeout: time.Second * 20,
	}
	traceMsg("http.Client timeout set to 20 seconds for release download")

	// Download requested release from Dojo's Github repo
	traceMsg(fmt.Sprintf("Downloading release from %+v", dwnURL))
	resp, err := ddClient.Get(dwnURL)
	if err != nil {
		traceMsg(fmt.Sprintf("Error downloading from %+v", dwnURL))
		traceMsg(fmt.Sprintf("Error downloading was: %+v", err))
		return err
	}
	defer resp.Body.Close()
	// TODO: Check for 200 status before moving on
	traceMsg(fmt.Sprintf("Status of http.Client response was %+v", resp.Status))

	// Create the file handle
	traceMsg("Creating file for downloaded tarball")
	out, err := os.Create(tarball)
	if err != nil {
		traceMsg(fmt.Sprintf("Error creating tarball was: %+v", err))
		return err
	}

	// Write the content downloaded into the file
	traceMsg("Writing downloaded content to tarball file")
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		traceMsg(fmt.Sprintf("Error writing file contents was: %+v", err))
		return err
	}

	// Extract the tarball to create the Dojo source directory
	traceMsg("Extracting tarball into the Dojo source directory")
	tb, err := os.Open(tarball)
	if err != nil {
		traceMsg(fmt.Sprintf("Error openging tarball was: %+v", err))
		return err
	}
	err = util.Untar(i.Root, tb)
	if err != nil {
		traceMsg(fmt.Sprintf("Error extracting tarball was: %+v", err))
		return err
	}

	// Remane source directory to the non-versioned name
	traceMsg("Renaming source directory to the non-versioned name")
	oldPath := filepath.Join(i.Root, "django-DefectDojo-"+i.Version)
	newPath := filepath.Join(i.Root, i.Source)
	err = os.Rename(oldPath, newPath)
	if err != nil {
		traceMsg(fmt.Sprintf("Error renaming Dojo source directory was: %+v", err))
		return err
	}

	// Successfully extracted the file, return nil
	statusMsg("Successfully downloaded and extracted the DefectDojo release file")
	return nil
}

// Use go-git to checkout latest source - either from a specfic commit or HEAD on a branch
// and places it in the specified dojoSource directory (default is /opt/dojo)
func getDojoSource(i *config.InstallConfig) error {
	statusMsg("Downloading DefectDojo source as a branch or commit from the repo directly")

	// Create the directory to clone the source into if it doesn't exist already
	traceMsg("Creating source directory if it doesn't exist already")
	srcPath := filepath.Join(i.Root, i.Source)
	_, err := os.Stat(srcPath)
	if err != nil {
		// Source directory doesn't exist
		err = os.MkdirAll(srcPath, 0755)
		if err != nil {
			traceMsg(fmt.Sprintf("Error creating Dojo source directory was: %+v", err))
			// TODO: Better handle the case when the repo already exists at that path - maybe?
			return err
		}
	}

	// Check out a specific branch or commit - but only one of those
	// In the case that both commit and branch are set to non-empty strings,
	// the configured commit will win (aka only the commit alone will be done)
	traceMsg("Determing if a commit or branch will be checked out of the repo")
	if len(i.SourceCommit) > 0 {
		// Commit is set, so it will be used and branch ignored
		statusMsg(fmt.Sprintf("Dojo will be installed from commit %+v", i.SourceCommit))

		// Do the initial clone of DefectDojo from Github
		traceMsg(fmt.Sprintf("Intial clone of %+v", CloneURL))
		repo, err := git.PlainClone(srcPath, false, &git.CloneOptions{URL: CloneURL})
		if err != nil {
			traceMsg(fmt.Sprintf("Error cloning the DefectDojo repo was: %+v", err))
			return err
		}

		// Setup the working tree for checking out a particular commit
		traceMsg("Setting up the working tree to checkout the commit")
		wk, err := repo.Worktree()
		err = wk.Checkout(&git.CheckoutOptions{Hash: plumbing.NewHash(i.SourceCommit)})
		if err != nil {
			fmt.Printf("Error checking out was %+v\n", err)
			traceMsg(fmt.Sprintf("Error checking out was: %+v", err))
			return err
		}

	} else {
		if len(i.SourceBranch) == 0 {
			// Handle the case that both source commit and branch are wonky
			err = fmt.Errorf("Both source commit and branch have empty or nonsensical values configured.\n"+
				"  Source commit was configured as %s and branch was configured as %s", i.SourceCommit, i.SourceBranch)
			traceMsg(fmt.Sprintf("Error checking out Dojo source was: %+v", err))
			return err
		}
		statusMsg(fmt.Sprintf("DefectDojo will be installed from branch %+v", i.SourceBranch))

		// Check out a specfic branch
		// Note: Branch and tag references are a bit odd, see https://github.com/src-d/go-git/blob/master/_examples/branch/main.go#L33
		//       However, the installer appends the necessary string to the 'normal' branch name
		traceMsg(fmt.Sprintf("Checking out branch %+v", i.SourceBranch))
		_, err = git.PlainClone(srcPath, false, &git.CloneOptions{
			URL:           CloneURL,
			ReferenceName: plumbing.ReferenceName("refs/heads/" + i.SourceBranch),
			SingleBranch:  true,
		})
		if err != nil {
			traceMsg(fmt.Sprintf("Error checking out branch was: %+v", err))
			return err
		}

	}

	// Successfully checked out the configured source, return nil
	statusMsg("Successfully checked out the configured DefectDojo source")
	return nil
}

func main() {
	// Setup viper config
	viper.AddConfigPath(".")
	viper.SetConfigName("dojoConfig")
	var conf config.DojoConfig

	// Setup ENV variables
	viper.SetEnvPrefix("DD")
	replace := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replace)
	viper.AutomaticEnv()

	// Read the default config file dojoConfig.yml
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Println("")
		fmt.Println("Unable to read the godojo config file (dojoConfig.yml), exiting install")
		os.Exit(1)
	}
	// Marshall the config values into the DojoConfig struct
	err = viper.Unmarshal(&conf)
	if err != nil {
		fmt.Println("")
		fmt.Println("Unable to set the config values based on config file and ENV variables, exiting install")
		os.Exit(1)
	}

	// Setup output and logging levels and print the DefectDojo banner if needed
	Quiet = conf.Install.Quiet
	TraceOn = conf.Install.Trace
	if !Quiet {
		dojoBanner()
	}

	// Check that user is root for the installer or run with "sudo godojo"
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	if usr.Uid != "0" {
		fmt.Println("")
		fmt.Println("##############################################################################")
		fmt.Println("  ERROR: This program must be run as root or with sudo\n  Please correct and run installer again")
		fmt.Println("##############################################################################")
		fmt.Println("")
		fmt.Println("DEBUG => [NOT] Exiting install")
		// TODO: Remove DEBUG below and above
		// DEBUG os.Exit(1)
	}

	// Setup logging for the installer
	n := time.Now()
	when := strconv.Itoa(int(n.UnixNano()))
	logName := "dojo-install_" + when + ".log"
	logPath := path.Join(logLocation, logName)
	// Create the logs directory if it does not exist
	_, err = os.Stat(logPath)
	if err != nil {
		// logs directory doesn't exist
		err = os.MkdirAll(logLocation, 0755)
		if err != nil {
			// Can't create logs directory for some reason, exit after showing error
			fmt.Println("")
			fmt.Println("##############################################################################")
			fmt.Printf("  Error creating godojo installer logging directory was %+v\n", err)
			fmt.Println("    Installation requires a logging directory.  Either create one in the same")
			fmt.Println("    directory as the godojo installer or correct the error above.")
			fmt.Println("##############################################################################")
			fmt.Println("")
			fmt.Println("Exiting install")
			os.Exit(1)
		}
	}

	// Create log file for the install
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("")
		fmt.Println("##############################################################################")
		fmt.Printf("  ERROR: Failed to open log file %s.  Error was:\n    %+v\n", logPath, err)
		fmt.Println("##############################################################################")
		fmt.Println("")
		fmt.Println("Log files are required for the install, exiting install")
		os.Exit(1)
	}
	// Log everthing to the specificied log file location
	logSetup(logFile)

	// Logging is setup, start using statusMsg and errorMsg functions for output
	traceMsg("Logging established, trace log begins here")
	sectionMsg("Starting the dojo install at " + n.Format("Mon Jan 2, 2006 15:04:05 MST"))

	// Write out the runtime config based on the net of the config file + ENV variables
	traceMsg("Writing out the runtime install configuration file")
	err = viper.WriteConfigAs("runtime-install-config.yml")
	if err != nil {
		errorMsg(fmt.Sprintf("Error from writing the runtime config was: %+v", err))
		os.Exit(1)
	}

	sectionMsg("Downloading the source for DefectDojo")

	// Determine if a release or Dojo source will be installed
	traceMsg(fmt.Sprintf("Determing if this is a source or release install: SourceInstall is %+v", conf.Install.SourceInstall))
	if conf.Install.SourceInstall {
		// Checkout the Dojo source directly from Github
		traceMsg("Dojo will be installed from source")

		err = getDojoSource(&conf.Install)
		if err != nil {
			errorMsg(fmt.Sprintf("Error attempting to install Dojo source was:\n    %+v", err))
			os.Exit(1)
		}
	} else {
		// Download Dojo source as a Github release tarball
		traceMsg("Dojo will be installed from a release tarball")

		err = getDojoRelease(&conf.Install)
		if err != nil {
			errorMsg(fmt.Sprintf("Error attempting to install Dojo from a release tarball was:\n    %+v", err))
			os.Exit(1)
		}

	}

	// Start stub'ing out stuff
	// Look at setup.bash's high-level workflow
	fmt.Println("\n\nSuccefully reached the end of main")
}
