// Package database contiene el esquema SQL de la aplicación.
package database

import _ "embed"

// Schema es el DDL completo de la base de datos, embebido en el binario.
//
//go:embed schema.sql
var Schema string
