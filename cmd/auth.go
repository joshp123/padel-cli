package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"padel-cli/storage"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func authCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
	}

	cmd.AddCommand(authLoginCmd())
	cmd.AddCommand(authStatusCmd())
	cmd.AddCommand(authLogoutCmd())
	return cmd
}

func authLoginCmd() *cobra.Command {
	var email string
	var password string
	var authFile string
	authFileDefault := os.Getenv("PADEL_AUTH_FILE")

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Login to Playtomic",
		RunE: func(cmd *cobra.Command, args []string) error {
			if authFile != "" {
				fileEmail, filePassword, err := readAuthFile(authFile)
				if err != nil {
					return err
				}
				if email == "" {
					email = fileEmail
				}
				if password == "" {
					password = filePassword
				}
			}

			if email == "" {
				fmt.Print("Email: ")
				reader := bufio.NewReader(os.Stdin)
				value, err := reader.ReadString('\n')
				if err != nil {
					return err
				}
				email = strings.TrimSpace(value)
			}
			if password == "" {
				fmt.Print("Password: ")
				bytes, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Println()
				if err != nil {
					return err
				}
				password = strings.TrimSpace(string(bytes))
			}
			if email == "" || password == "" {
				return fmt.Errorf("email and password are required")
			}

			ctx := context.Background()
			resp, err := client.Login(ctx, email, password)
			if err != nil {
				return err
			}

			creds := storage.Credentials{
				AccessToken:            resp.AccessToken,
				AccessTokenExpiration:  resp.AccessTokenExpiration,
				RefreshToken:           resp.RefreshToken,
				RefreshTokenExpiration: resp.RefreshTokenExpiration,
				UserID:                 resp.UserID,
				Email:                  email,
			}
			if err := storage.SaveCredentials(&creds); err != nil {
				return err
			}

			fmt.Printf("Logged in as %s.\n", email)
			return nil
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "Email address")
	cmd.Flags().StringVar(&password, "password", "", "Password")
	cmd.Flags().StringVar(&authFile, "auth-file", authFileDefault, "Load credentials from file (default: $PADEL_AUTH_FILE)")
	return cmd
}

func authStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check auth status",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, err := storage.LoadCredentials()
			if err != nil {
				return err
			}
			if creds == nil || creds.AccessToken == "" {
				fmt.Println("Not logged in.")
				return nil
			}

			expired := creds.AccessTokenExpired(time.Now())
			if expired {
				fmt.Printf("Token expired for %s. Run 'padel auth login' to re-authenticate.\n", creds.Email)
				return nil
			}
			fmt.Printf("Logged in as %s.\n", creds.Email)
			fmt.Printf("Token expires: %s\n", creds.AccessTokenExpiration)
			return nil
		},
	}

	return cmd
}

func authLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Logout and clear credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := storage.ClearCredentials(); err != nil {
				return err
			}
			fmt.Println("Logged out.")
			return nil
		},
	}

	return cmd
}

func readAuthFile(path string) (string, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var email string
	var password string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch line {
		case "[username]":
			if scanner.Scan() {
				email = strings.TrimSpace(scanner.Text())
			}
		case "[password]":
			if scanner.Scan() {
				password = strings.TrimSpace(scanner.Text())
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", "", err
	}
	return email, password, nil
}
