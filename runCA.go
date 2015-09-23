package main

import (
	"fmt"
	"github.com/jawher/mow.cli"
	"github.com/olekukonko/tablewriter"
	"os"
	"strconv"
)

func caCmd(cmd *cli.Cmd) {
	cmd.Command("new", "Create a new CA", caNewCmd)
	cmd.Command("list", "List CAs", caListCmd)
	cmd.Command("show", "Show a CA", caShowCmd)
	cmd.Command("update", "Update an existing CA", caUpdateCmd)
	cmd.Command("delete", "Delete a CA", caDeleteCmd)
}

func caNewCmd(cmd *cli.Cmd) {
	cmd.Spec = "NAME [OPTIONS]"

	params := NewCAParams()
	params.name = cmd.StringArg("NAME", "", "name of CA")

	params.certFile = cmd.StringOpt("cert", "", "certificate PEM file")
	params.keyFile = cmd.StringOpt("key", "", "key PEM file")
	params.tags = cmd.StringOpt("tags", "NAME", "comma separated list of tags")
	params.caExpiry = cmd.IntOpt("ca-expiry", 365, "CA expiry period in days")
	params.certExpiry = cmd.IntOpt("cert-expiry", 90, "Certificate expiry period in days")
	params.keyType = cmd.StringOpt("key-type", "ec", "Key type (ec or rsa)")
	params.dnLocality = cmd.StringOpt("dn-l", "", "Locality for DN scope")
	params.dnState = cmd.StringOpt("dn-st", "", "State/province for DN scope")
	params.dnOrg = cmd.StringOpt("dn-o", "", "Organization for DN scope")
	params.dnOrgUnit = cmd.StringOpt("dn-ou", "", "Organizational unit for DN scope")
	params.dnCountry = cmd.StringOpt("dn-c", "", "Country for DN scope")
	params.dnStreet = cmd.StringOpt("dn-street", "", "Street for DN scope")
	params.dnPostal = cmd.StringOpt("dn-postal", "", "PostalCode for DN scope")

	cmd.Action = func() {
		initLogging(*logLevel, *logging)
		defer logger.Close()
		env := new(Environment)
		env.logger = logger

		cont, err := NewCAController(env)
		if err != nil {
			env.Fatal(err)
		}

		ca, err := cont.New(params)
		if err != nil {
			env.Fatal(err)
		}
		env.logger.Info("Created")

		if ca != nil {
			table := tablewriter.NewWriter(os.Stdout)
			table.SetAlignment(tablewriter.ALIGN_LEFT)

			caData := [][]string{
				[]string{"Id", ca.Id()},
				[]string{"Name", ca.Name()},
			}

			table.AppendBulk(caData)
			env.logger.Flush()
			table.Render()
		}

	}
}

func caListCmd(cmd *cli.Cmd) {
	params := NewCAParams()

	cmd.Action = func() {

		initLogging(*logLevel, *logging)
		defer logger.Close()
		env := new(Environment)
		env.logger = logger

		cont, err := NewCAController(env)
		if err != nil {
			env.Fatal(err)
		}

		cas, err := cont.List(params)
		if err != nil {
			env.Fatal(err)
		}

		table := tablewriter.NewWriter(os.Stdout)
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetHeader([]string{"Name", "Id"})
		for _, ca := range cas {
			table.Append([]string{ca.Name(), ca.Id()})
		}

		env.logger.Flush()
		table.Render()
	}
}

func caShowCmd(cmd *cli.Cmd) {
	cmd.Spec = "NAME [OPTIONS]"

	params := NewCAParams()
	params.name = cmd.StringArg("NAME", "", "name of CA")

	params.export = cmd.StringOpt("export", "", "tar.gz export to file")
	params.private = cmd.BoolOpt("private", false, "show/export private data")

	cmd.Action = func() {
		initLogging(*logLevel, *logging)
		defer logger.Close()
		env := new(Environment)
		env.logger = logger

		cont, err := NewCAController(env)
		if err != nil {
			env.Fatal(err)
		}

		ca, err := cont.Show(params)
		if err != nil {
			env.Fatal(err)
		}

		if ca != nil {
			table := tablewriter.NewWriter(os.Stdout)
			table.SetAlignment(tablewriter.ALIGN_LEFT)

			caData := [][]string{
				[]string{"ID", ca.Id()},
				[]string{"Name", ca.Name()},
				[]string{"Key type", ca.Data.Body.KeyType},
				[]string{"CA expiry period (days)", strconv.Itoa(ca.Data.Body.CAExpiry)},
				[]string{"Cert expiry period (days)", strconv.Itoa(ca.Data.Body.CertExpiry)},
				[]string{"Country DN scope", ca.Data.Body.DNScope.Country},
				[]string{"Organization DN scope", ca.Data.Body.DNScope.Organization},
				[]string{"Organizational unit DN scope", ca.Data.Body.DNScope.OrganizationalUnit},
				[]string{"Locality DN scope", ca.Data.Body.DNScope.Locality},
				[]string{"Province DN scope", ca.Data.Body.DNScope.Province},
				[]string{"Street address DN scope", ca.Data.Body.DNScope.StreetAddress},
				[]string{"Postal code DN scope", ca.Data.Body.DNScope.PostalCode},
			}

			table.AppendBulk(caData)
			env.logger.Flush()
			table.Render()

			fmt.Println("")
			fmt.Printf("Certificate:\n%s\n", ca.Data.Body.Certificate)

			if *params.private {
				fmt.Printf("Private key:\n%s\n", ca.Data.Body.PrivateKey)
			}
		}
	}
}

func caUpdateCmd(cmd *cli.Cmd) {
	cmd.Spec = "NAME [OPTIONS]"

	params := NewCAParams()
	params.name = cmd.StringArg("NAME", "", "name of CA")

	params.certFile = cmd.StringOpt("cert", "", "certificate PEM file")
	params.keyFile = cmd.StringOpt("key", "", "key PEM file")
	params.tags = cmd.StringOpt("tags", "", "comma separated list of tags")
	params.caExpiry = cmd.IntOpt("ca-expiry", 0, "CA expiry period in days")
	params.certExpiry = cmd.IntOpt("cert-expiry", 0, "Certificate expiry period in days")
	params.dnLocality = cmd.StringOpt("dn-l", "", "Locality for DN scope")
	params.dnState = cmd.StringOpt("dn-st", "", "State/province for DN scope")
	params.dnOrg = cmd.StringOpt("dn-o", "", "Organization for DN scope")
	params.dnOrgUnit = cmd.StringOpt("dn-ou", "", "Organizational unit for DN scope")
	params.dnCountry = cmd.StringOpt("dn-c", "", "Country for DN scope")
	params.dnStreet = cmd.StringOpt("dn-street", "", "Street for DN scope")
	params.dnPostal = cmd.StringOpt("dn-postal", "", "PostalCode for DN scope")

	cmd.Action = func() {
		initLogging(*logLevel, *logging)
		defer logger.Close()
		env := new(Environment)
		env.logger = logger

		cont, err := NewCAController(env)
		if err != nil {
			env.Fatal(err)
		}

		env.logger.Info("Updating CA")
		if err := cont.Update(params); err != nil {
			env.Fatal(err)
		}
		env.logger.Info("Complete")

	}
}

func caDeleteCmd(cmd *cli.Cmd) {
	cmd.Spec = "NAME [OPTIONS]"

	params := NewCAParams()
	params.name = cmd.StringArg("NAME", "", "name of CA")

	params.confirmDelete = cmd.StringOpt("confirm-delete", "", "reason for deleting CA")

	cmd.Action = func() {
		initLogging(*logLevel, *logging)
		defer logger.Close()
		env := new(Environment)
		env.logger = logger

		cont, err := NewCAController(env)
		if err != nil {
			env.Fatal(err)
		}

		env.logger.Info("Deleting CA")
		if err := cont.Delete(params); err != nil {
			env.Fatal(err)
		}
		env.logger.Info("Complete")
	}
}
