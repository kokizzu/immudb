/*
Copyright 2019-2020 vChain, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package immudb

import (
	"os"
	"path/filepath"

	"github.com/codenotary/immudb/cmd/docs/man"
	c "github.com/codenotary/immudb/cmd/helper"
	"github.com/codenotary/immudb/cmd/version"
	"github.com/codenotary/immudb/pkg/logger"
	"github.com/codenotary/immudb/pkg/server"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/spf13/viper"
	daem "github.com/takama/daemon"
)

var o = c.Options{}

func init() {
	cobra.OnInitialize(func() { o.InitConfig("immudb") })
}

// NewCmd ...
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "immudb",
		Short: "immudb - the lightweight, high-speed immutable database for systems and applications",
		Long: `immudb - the lightweight, high-speed immutable database for systems and applications.

Environment variables:
  IMMUDB_DIR=.
  IMMUDB_NETWORK=tcp
  IMMUDB_ADDRESS=127.0.0.1
  IMMUDB_PORT=3322
  IMMUDB_DBNAME=immudb
  IMMUDB_PIDFILE=
  IMMUDB_LOGFILE=
  IMMUDB_MTLS=false
  IMMUDB_AUTH=true
  IMMUDB_DETACHED=false
  IMMUDB_CONSISTENCY_CHECK=true
  IMMUDB_PKEY=./tools/mtls/3_application/private/localhost.key.pem
  IMMUDB_CERTIFICATE=./tools/mtls/3_application/certs/localhost.cert.pem
  IMMUDB_CLIENTCAS=./tools/mtls/2_intermediate/certs/ca-chain.cert.pem
  IMMUDB_DEVMODE=true
  IMMUDB_MAINTENANCE=false
  IMMUDB_ADMIN_PASSWORD=immudb`,
		DisableAutoGenTag: true,
		RunE:              Immudb,
	}

	setupFlags(cmd, server.DefaultOptions(), server.DefaultMTLsOptions())

	if err := bindFlags(cmd); err != nil {
		c.QuitToStdErr(err)
	}
	setupDefaults(server.DefaultOptions(), server.DefaultMTLsOptions())

	cmd.AddCommand(man.Generate(cmd, "immudb", "./cmd/docs/man/immudb"))
	cmd.AddCommand(version.VersionCmd())

	return cmd
}

// Immudb ...
func Immudb(cmd *cobra.Command, args []string) (err error) {
	var options server.Options
	if options, err = parseOptions(cmd); err != nil {
		return err
	}
	immuServer := server.
		DefaultServer().
		WithOptions(options)
	if options.Logfile != "" {
		if flogger, file, err := logger.NewFileLogger("immudb ", options.Logfile); err == nil {
			defer func() {
				if err = file.Close(); err != nil {
					c.QuitToStdErr(err)
				}
			}()
			immuServer.WithLogger(flogger)
		} else {
			c.QuitToStdErr(err)
		}
	}

	if options.Detached {
		c.Detached()
	}

	var d daem.Daemon
	if d, err = daem.New("immudb", "immudb", "immudb"); err != nil {
		c.QuitToStdErr(err)
	}

	service := server.Service{
		ImmuServer: *immuServer,
	}

	d.Run(service)

	return nil
}

func parseOptions(cmd *cobra.Command) (options server.Options, err error) {
	dir, err := c.ResolvePath(viper.GetString("dir"), true)
	if err != nil {
		return options, err
	}
	port := viper.GetInt("port")
	address := viper.GetString("address")
	// config file came only from arguments or default folder
	if o.CfgFn, err = cmd.Flags().GetString("config"); err != nil {
		return server.Options{}, err
	}
	if err != nil {
		return options, err
	}
	pidfile, err := c.ResolvePath(viper.GetString("pidfile"), true)
	if err != nil {
		return options, err
	}
	logfile, err := c.ResolvePath(viper.GetString("logfile"), true)
	if err != nil {
		return options, err
	}
	mtls := viper.GetBool("mtls")
	auth := viper.GetBool("auth")
	noHistograms := viper.GetBool("no-histograms")
	detached := viper.GetBool("detached")
	consistencyCheck := viper.GetBool("consistency-check")
	certificate, err := c.ResolvePath(viper.GetString("certificate"), true)
	if err != nil {
		return options, err
	}
	pkey, err := c.ResolvePath(viper.GetString("pkey"), true)
	if err != nil {
		return options, err
	}
	clientcas, err := c.ResolvePath(viper.GetString("clientcas"), true)
	if err != nil {
		return options, err
	}
	devMode := viper.GetBool("devmode")
	adminPassword := viper.GetString("admin-password")
	maintenance := viper.GetBool("maintenance")

	options = server.
		DefaultOptions().
		WithDir(dir).
		WithPort(port).
		WithAddress(address).
		WithConfig(o.CfgFn).
		WithPidfile(pidfile).
		WithLogfile(logfile).
		WithMTLs(mtls).
		WithAuth(auth).
		WithNoHistograms(noHistograms).
		WithDetached(detached).
		WithCorruptionCheck(consistencyCheck).
		WithDevMode(devMode).
		WithAdminPassword(adminPassword).
		WithMaintenance(maintenance)
	if mtls {
		// todo https://golang.org/src/crypto/x509/root_linux.go
		options.MTLsOptions = server.DefaultMTLsOptions().
			WithCertificate(certificate).
			WithPkey(pkey).
			WithClientCAs(clientcas)
	}
	return options, nil
}

func setupFlags(cmd *cobra.Command, options server.Options, mtlsOptions server.MTLsOptions) {
	cmd.Flags().String("dir", options.Dir, "data folder")
	cmd.Flags().IntP("port", "p", options.Port, "port number")
	cmd.Flags().StringP("address", "a", options.Address, "bind address")
	cmd.Flags().StringVar(&o.CfgFn, "config", "", "config file (default path are configs or $HOME. Default filename is immudb.ini)")
	cmd.Flags().String("pidfile", options.Pidfile, "pid path with filename. E.g. /var/run/immudb.pid")
	cmd.Flags().String("logfile", options.Logfile, "log path with filename. E.g. /tmp/immudb/immudb.log")
	cmd.Flags().BoolP("mtls", "m", options.MTLs, "enable mutual tls")
	cmd.Flags().BoolP("auth", "s", options.MTLs, "enable auth")
	cmd.Flags().Bool("no-histograms", options.MTLs, "disable collection of histogram metrics like query durations")
	cmd.Flags().Bool("consistency-check", options.CorruptionCheck, "enable consistency check monitor routine. To disable: --consistency-check=false")
	cmd.Flags().BoolP(c.DetachedFlag, c.DetachedShortFlag, options.Detached, "run immudb in background")
	cmd.Flags().String("certificate", mtlsOptions.Certificate, "server certificate file path")
	cmd.Flags().String("pkey", mtlsOptions.Pkey, "server private key path")
	cmd.Flags().String("clientcas", mtlsOptions.ClientCAs, "clients certificates list. Aka certificate authority")
	cmd.Flags().Bool("devmode", options.DevMode, "enable dev mode: accept remote connections without auth")
	cmd.Flags().String("admin-password", options.AdminPassword, "admin password (default is 'immu') as plain-text or base64 encoded (must be prefixed with 'enc:' if it is encoded)")
	cmd.Flags().Bool("maintenance", options.GetMaintenance(), "override the authentication flag")
}

func bindFlags(cmd *cobra.Command) error {
	if err := viper.BindPFlag("dir", cmd.Flags().Lookup("dir")); err != nil {
		return err
	}
	if err := viper.BindPFlag("port", cmd.Flags().Lookup("port")); err != nil {
		return err
	}
	if err := viper.BindPFlag("address", cmd.Flags().Lookup("address")); err != nil {
		return err
	}
	if err := viper.BindPFlag("dbname", cmd.Flags().Lookup("dbname")); err != nil {
		return err
	}
	if err := viper.BindPFlag("pidfile", cmd.Flags().Lookup("pidfile")); err != nil {
		return err
	}
	if err := viper.BindPFlag("logfile", cmd.Flags().Lookup("logfile")); err != nil {
		return err
	}
	if err := viper.BindPFlag("mtls", cmd.Flags().Lookup("mtls")); err != nil {
		return err
	}
	if err := viper.BindPFlag("auth", cmd.Flags().Lookup("auth")); err != nil {
		return err
	}
	if err := viper.BindPFlag("no-histograms", cmd.Flags().Lookup("no-histograms")); err != nil {
		return err
	}
	if err := viper.BindPFlag("consistency-check", cmd.Flags().Lookup("consistency-check")); err != nil {
		return err
	}
	if err := viper.BindPFlag("detached", cmd.Flags().Lookup("detached")); err != nil {
		return err
	}
	if err := viper.BindPFlag("certificate", cmd.Flags().Lookup("certificate")); err != nil {
		return err
	}
	if err := viper.BindPFlag("pkey", cmd.Flags().Lookup("pkey")); err != nil {
		return err
	}
	if err := viper.BindPFlag("clientcas", cmd.Flags().Lookup("clientcas")); err != nil {
		return err
	}
	if err := viper.BindPFlag("devmode", cmd.Flags().Lookup("devmode")); err != nil {
		return err
	}
	if err := viper.BindPFlag("admin-password", cmd.Flags().Lookup("admin-password")); err != nil {
		return err
	}
	if err := viper.BindPFlag("maintenance", cmd.Flags().Lookup("maintenance")); err != nil {
		return err
	}
	return nil
}

func setupDefaults(options server.Options, mtlsOptions server.MTLsOptions) {
	viper.SetDefault("dir", options.Dir)
	viper.SetDefault("port", options.Port)
	viper.SetDefault("address", options.Address)
	viper.SetDefault("pidfile", options.Pidfile)
	viper.SetDefault("logfile", options.Logfile)
	viper.SetDefault("mtls", options.MTLs)
	viper.SetDefault("auth", options.GetAuth())
	viper.SetDefault("no-histograms", options.NoHistograms)
	viper.SetDefault("consistency-check", options.CorruptionCheck)
	viper.SetDefault("detached", options.Detached)
	viper.SetDefault("certificate", mtlsOptions.Certificate)
	viper.SetDefault("pkey", mtlsOptions.Pkey)
	viper.SetDefault("clientcas", mtlsOptions.ClientCAs)
	viper.SetDefault("devmode", options.DevMode)
	viper.SetDefault("admin-password", options.AdminPassword)
	viper.SetDefault("maintenance", options.GetMaintenance())
}

// InstallManPages installs man pages
func InstallManPages() error {
	header := &doc.GenManHeader{
		Title:   "immuadmin service",
		Section: "1",
		Source:  "Generated by immuadmin installer",
	}
	dir := c.LinuxManPath

	_ = os.Mkdir(dir, os.ModePerm)
	err := doc.GenManTree(NewCmd(), header, dir)
	if err != nil {
		return err
	}
	return nil
}

// UnistallManPages uninstalls man pages
func UnistallManPages() error {
	return os.Remove(filepath.Join(c.LinuxManPath, "immudb.1"))
}
