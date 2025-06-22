package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/fatih/color"
	"github.com/kolo/xmlrpc"
	"github.com/spf13/cobra"
)

type BackupMetadata struct {
	OdooVersion string   `json:"odoo_version"`
	DBName      string   `json:"db_name"`
	BackupDate  string   `json:"backup_date"`
	Addons      []string `json:"addons"`
}

const odooURL = "http://localhost:8069"

var (
	dbName    string
	backupDir string
	addonsDir string
)

// backupCmd represents the backup command
var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup Odoo container",
	Long: `This command allows you to backup your Odoo container, including the database, filestore, and addons.
	It creates a tar.gz archive with the name of the database and today's date.`,
	Run: func(cmd *cobra.Command, args []string) {
		// fmt.Println("backup command executed")
		// Call the backupOdoo function
		err := backupOdoo(cmd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	RootCmd.AddCommand(backupCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	backupCmd.PersistentFlags().StringVar(&dbName, "db", "", "Database name")
	backupCmd.PersistentFlags().StringVar(&backupDir, "backup-dir", "/tmp/odoo-backup", "Backup directory")
	backupCmd.PersistentFlags().StringVar(&addonsDir, "addons-dir", "/mnt/extra-addons", "Addons directory")
	backupCmd.Flags().Bool("rm-backup-dir", true, "Delete backup folder after create archive")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// backupCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func getOdooVersion(odooURL string) (string, error) {
	client, err := xmlrpc.NewClient(fmt.Sprintf("%s/xmlrpc/2/common", odooURL), nil)
	if err != nil {
		return "", err
	}

	var versionInfo map[string]interface{}
	err = client.Call("version", nil, &versionInfo)
	if err != nil {
		return "", err
	}

	if version, ok := versionInfo["server_version"].(string); ok {
		return version, nil
	}
	return "", fmt.Errorf("server_version not found in response")
}

func createMetadata(backupPath string, dbName string, addonsDir string, odooVersion string) error {
	// Get the current date
	backupDate := time.Now().Format("2006-01-02 15:04:05")

	// Get the list of addons
	var addons []string
	files, err := os.ReadDir(addonsDir)
	if err != nil {
		return fmt.Errorf("error reading addons directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			addons = append(addons, file.Name())
		}
	}

	// Create the metadata struct
	metadata := BackupMetadata{
		OdooVersion: odooVersion,
		DBName:      dbName,
		BackupDate:  backupDate,
		Addons:      addons,
	}

	// Create the JSON file
	metadataFile := fmt.Sprintf("%s/metadata.json", backupPath)
	file, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling metadata to JSON: %w", err)
	}

	err = os.WriteFile(metadataFile, file, 0644)
	if err != nil {
		return fmt.Errorf("error writing metadata file: %w", err)
	}

	color.HiMagenta("Metadata file created at: %s", metadataFile)
	return nil
}

func backupOdoo(cmd *cobra.Command) error {
	// Implement backup logic here
	color.HiMagenta("==================================")
	color.HiMagenta("==== Starting Odoo backup... =====")
	color.HiMagenta("==================================")

	// 1. Get Odoo information via XML-RPC
	fmt.Println("[*] Getting Odoo information via XML-RPC...")
	version, err := getOdooVersion(odooURL)
	if err != nil {
		return fmt.Errorf("error getting Odoo version: %w", err)

	}
	color.HiYellow("Odoo Version: %s", version)

	// 2. Create backup directory
	fmt.Printf("[*] Creating backup directory at %s", backupDir)
	timestamp := time.Now().Format("200601021504")
	backupPath := fmt.Sprintf("%s/%s-%s", backupDir, dbName, timestamp)
	color.Cyan("Backup path: %s", backupPath)
	err = os.MkdirAll(backupPath, 0755)
	if err != nil {
		return fmt.Errorf("error creating backup directory: %w", err)
	}

	// 3. Create subdirectories
	fmt.Println("[*] Creating subdirectories...")
	databasePath := fmt.Sprintf("%s/database", backupPath)
	filestorePath := fmt.Sprintf("%s/filestore", backupPath)
	addonsPath := fmt.Sprintf("%s/addons", backupPath)

	err = os.MkdirAll(databasePath, 0755)
	if err != nil {
		return fmt.Errorf("error creating database directory: %w", err)
	}
	color.HiMagenta("Database directory created at: %s", databasePath)

	err = os.MkdirAll(filestorePath, 0755)
	if err != nil {
		return fmt.Errorf("error creating filestore directory: %w", err)
	}
	color.HiMagenta("Filestore directory created at: %s", filestorePath)

	err = os.MkdirAll(addonsPath, 0755)
	if err != nil {
		return fmt.Errorf("error creating addons directory: %w", err)
	}
	color.HiMagenta("Addons directory created at: %s", addonsPath)

	// Create metadata file
	fmt.Println("[*] Creating metadata file...")
	err = createMetadata(backupPath, dbName, addonsDir, version)
	if err != nil {
		return fmt.Errorf("error creating metadata file: %w", err)
	}

	// 4. Dump the database
	err = dumpDatabase(dbName, backupPath)
	if err != nil {
		return fmt.Errorf("error dumping database: %w", err)
	}

	// 5. Copy the filestore
	if err == nil {
		err = copyFilestore(dbName, filestorePath)
		if err != nil {
			return fmt.Errorf("error copying filestore: %w", err)
		}

		// 7. Copy the addons
		err = copyAddons(addonsDir, addonsPath)
		if err != nil {
			return fmt.Errorf("error copying addons: %w", err)
		}

		// 8. Create the metadata file
		err = createMetadata(backupPath, dbName, addonsDir, version)
		if err != nil {
			return fmt.Errorf("error creating metadata file: %w", err)
		}
		color.HiMagenta("Metadata file created successfully at: %s/metadata.json", backupPath)

		// 9. Create the tar.gz archive
		fmt.Println("[*] Creating archive...")
		archiveName := fmt.Sprintf("%s_%s.tar.gz", dbName, time.Now().Format("20060102"))
		archiveFile := fmt.Sprintf("%s/%s", backupDir, archiveName)
		err = createArchive(backupPath, archiveFile)
		if err != nil {
			return fmt.Errorf("error creating archive: %w", err)
		}

		// 10. Delete backup folder
		deleteAfterArchive, err := cmd.Flags().GetBool("rm-backup-dir")
		if err != nil {
			return fmt.Errorf("error getting rm-backup-dir flag: %w", err)
		}

		if deleteAfterArchive {
			color.HiMagenta("[*] Deleting backup folder...")
			err = os.RemoveAll(backupPath)
			if err != nil {
				return fmt.Errorf("error deleting backup folder: %w", err)
			}
		} else {
			color.HiYellow("Skipping backup folder deletion...")
		}
		color.HiGreen("************************************************")
		color.HiGreen("Odoo backup completed successfully!")
		color.HiGreen("************************************************")
		fmt.Println()
		color.HiGreen("[NOTE]: You can copy the archive file to your local machine using:")
		color.HiGreen("docker cp <container_id>:%s .", archiveFile)
		fmt.Println()
	}

	return nil
}

func dumpDatabase(dbName string, backupPath string) error {
	// Implement database dumping logic here
	color.Yellow("[*] Dumping database: %s\n", dbName)

	// 1. Get environment variables
	fmt.Println("[*] Getting environment variables...")
	host := os.Getenv("HOST")
	port := os.Getenv("PORT")
	user := os.Getenv("USER")
	password := os.Getenv("PASSWORD")
	os.Setenv("PGPASSWORD", password) // Set PGPASSWORD for pg_dump

	if host == "" || port == "" || user == "" || password == "" {
		return fmt.Errorf("missing environment variables: HOST, PORT, USER, PASSWORD")
	}

	color.Cyan("HOST: %s, PORT: %s, USER: %s\n", host, port, user)

	// Check if pg_dump is available
	_, err := exec.LookPath("pg_dump")
	if err != nil {
		fmt.Fprintf(os.Stderr, "pg_dump not found: %s\n", err)
		return fmt.Errorf("pg_dump not found in PATH. Please install PostgreSQL and ensure pg_dump is in your PATH")
	}

	// Construct the pg_dump command
	cmd := exec.Command("pg_dump", "-h", host, "-p", port, "-U", user, "-F", "c", "-f", fmt.Sprintf("%s/%s_dump.sql", backupPath, dbName), dbName)
	fmt.Printf("[*] Executing command: %s\n", cmd.String())
	// Execute the command
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("error dumping database: %w", err)
	}

	color.HiMagenta("Database dumped successfully to: %s_dump.sql\n", dbName)
	return nil
}

func copyFilestore(dbName string, backupDir string) error {
	// Implement filestore copying logic here
	filestoreDir := fmt.Sprintf("/var/lib/odoo/filestore/%s", dbName)
	fmt.Println("[*] Copying filestore ...")

	// Construct the cp command
	cmd := exec.Command("cp", "-ra", filestoreDir, backupDir)

	// Execute the command
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error copying filestore: %w", err)
	}

	color.HiMagenta("Filestore copied successfully to %s\n", backupDir)
	return nil
}

func copyAddons(addonsDir string, backupDir string) error {
	// Implement addons copying logic here
	fmt.Println("[*] Copying addons ...")

	// Construct the cp command
	cmd := exec.Command("cp", "-ra", addonsDir, backupDir)

	// Execute the command
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error copying addons: %w", err)
	}

	color.HiMagenta("Addons copied successfully from: %s to: %s\n", addonsDir, backupDir)
	return nil
}

func createArchive(backupDir string, archivePath string) error {
	// Implement archive creation logic here
	fmt.Println("[*] Creating Backup archive file: ", archivePath)

	// Construct the tar command
	cmd := exec.Command("tar", "-czvf", archivePath, "-C", backupDir, ".")
	fmt.Println("[*] Executing command: ", cmd.String())

	// Execute the command
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error creating archive: %w", err)
	}

	color.HiMagenta("Archive created successfully: %s ", archivePath)
	return nil
}
