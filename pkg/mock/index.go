package mock

import (
	"html/template"
	"net/http"
	"sort"
	"strings"

	"github.com/sahina/cvt/pkg/cvt"
)

const indexHTML = `<!DOCTYPE html>
<html>
<head><title>CVT Mock Server</title>
<style>
body { font-family: -apple-system, system-ui, sans-serif; max-width: 900px; margin: 40px auto; padding: 0 20px; color: #333; }
h1 { border-bottom: 2px solid #eee; padding-bottom: 10px; }
h2 { color: #555; margin-top: 30px; }
table { width: 100%; border-collapse: collapse; margin-top: 10px; }
th, td { text-align: left; padding: 8px 12px; border-bottom: 1px solid #eee; }
th { background: #f8f8f8; font-weight: 600; }
.method { font-weight: bold; font-family: monospace; }
.path { font-family: monospace; }
.get { color: #2e7d32; } .post { color: #1565c0; } .put { color: #e65100; }
.patch { color: #6a1b9a; } .delete { color: #c62828; }
.empty { color: #999; font-style: italic; padding: 20px 0; }
</style></head>
<body>
<h1>CVT Mock Server</h1>
{{range .Schemas}}
<h2>{{.Name}}{{if .Version}} ({{.Version}}){{end}}</h2>
{{if .Endpoints}}
<table>
<tr><th>Method</th><th>Path</th><th>Summary</th></tr>
{{range .Endpoints}}<tr>
<td class="method {{.MethodLower}}">{{.Method}}</td>
<td class="path">{{.Path}}</td>
<td>{{.Summary}}</td>
</tr>{{end}}
</table>
{{else}}
<p class="empty">No endpoints defined</p>
{{end}}
{{end}}
</body></html>`

type schemaInfo struct {
	Name      string
	Version   string
	Endpoints []endpointInfo
}

type endpointInfo struct {
	Method      string
	MethodLower string
	Path        string
	Summary     string
}

// IndexHandler returns an http.Handler that renders the endpoint index page.
func IndexHandler(v *cvt.Validator, schemaIDs []string) http.Handler {
	tmpl := template.Must(template.New("index").Parse(indexHTML))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var schemas []schemaInfo

		for _, id := range schemaIDs {
			doc, ok := v.GetSchema(id)
			if !ok {
				continue
			}

			info := schemaInfo{Name: id}
			if doc.Info != nil {
				info.Version = doc.Info.Version
			}

			if doc.Paths != nil {
				for path, pathItem := range doc.Paths.Map() {
					for method, op := range pathItem.Operations() {
						ep := endpointInfo{
							Method:      method,
							MethodLower: strings.ToLower(method),
							Path:        path,
						}
						if op.Summary != "" {
							ep.Summary = op.Summary
						} else if op.OperationID != "" {
							ep.Summary = op.OperationID
						}
						info.Endpoints = append(info.Endpoints, ep)
					}
				}
			}

			sort.Slice(info.Endpoints, func(i, j int) bool {
				if info.Endpoints[i].Path == info.Endpoints[j].Path {
					return info.Endpoints[i].Method < info.Endpoints[j].Method
				}
				return info.Endpoints[i].Path < info.Endpoints[j].Path
			})

			schemas = append(schemas, info)
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tmpl.Execute(w, map[string]interface{}{"Schemas": schemas})
	})
}
