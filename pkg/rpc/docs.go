package rpc

import (
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strings"

	"github.com/CellsWeb/crpc-go/pkg/schema"
	"github.com/valyala/fasthttp"
)

const docsCSS = `
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
  :root {
    --bg: #0d1117;
    --surface: #161b22;
    --surface-hover: #1c2128;
    --border: #30363d;
    --text: #e6edf3;
    --muted: #8b949e;
    --accent: #58a6ff;
    --accent-dim: rgba(56, 139, 253, 0.15);
    --green: #3fb950;
    --green-dim: rgba(63, 185, 80, 0.15);
    --yellow: #d29922;
    --red: #f85149;
    --purple: #bc8cff;
    --code-bg: #161b22;
    --tag-bg: #21262d;
  }
  body {
    background: var(--bg);
    color: var(--text);
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif;
    font-size: 14px;
    line-height: 1.6;
    min-height: 100vh;
  }
  a { color: var(--accent); text-decoration: none; }
  a:hover { text-decoration: underline; }
  .layout { display: flex; min-height: 100vh; }
  .sidebar {
    width: 280px; min-width: 280px;
    background: var(--surface);
    border-right: 1px solid var(--border);
    padding: 24px 0;
    position: sticky; top: 0; height: 100vh;
    overflow-y: auto;
  }
  .sidebar-header {
    padding: 0 20px 16px;
    border-bottom: 1px solid var(--border);
    margin-bottom: 16px;
  }
  .sidebar-logo {
    font-size: 15px; font-weight: 700; color: var(--text);
    display: flex; align-items: center; gap: 8px;
    letter-spacing: -0.02em;
  }
  .sidebar-logo-badge {
    background: var(--accent-dim); color: var(--accent);
    font-size: 10px; padding: 2px 6px; border-radius: 4px; font-weight: 600;
  }
  .sidebar-ns {
    padding: 12px 20px 4px; font-size: 11px; font-weight: 700;
    color: var(--muted); text-transform: uppercase; letter-spacing: .08em;
  }
  .sidebar-fn {
    display: flex; align-items: center; justify-content: space-between;
    padding: 7px 20px 7px 28px;
    color: var(--text); font-size: 13px; font-family: ui-monospace, SFMono-Regular, SF Mono, Menlo, monospace;
    transition: background .12s ease;
  }
  .sidebar-fn:hover { background: var(--surface-hover); text-decoration: none; }
  .sidebar-fn.active { color: var(--accent); background: var(--accent-dim); font-weight: 600; }
  .sidebar-fn-badge {
    font-size: 10px; padding: 1px 5px; border-radius: 3px;
    background: var(--tag-bg); color: var(--muted); border: 1px solid var(--border);
    font-family: sans-serif;
  }
  .main { flex: 1; padding: 40px 48px; max-width: 1040px; }
  .page-header { margin-bottom: 32px; border-bottom: 1px solid var(--border); padding-bottom: 24px; }
  .page-breadcrumb { display: flex; align-items: center; gap: 8px; font-size: 13px; color: var(--muted); margin-bottom: 8px; }
  .page-title { font-size: 28px; font-weight: 700; letter-spacing: -0.02em; margin-bottom: 8px; font-family: ui-monospace, SFMono-Regular, SF Mono, Menlo, monospace; }
  .page-sub { color: var(--muted); font-size: 15px; }
  .badge {
    display: inline-flex; align-items: center; gap: 4px;
    padding: 3px 10px; border-radius: 12px;
    font-size: 11px; font-weight: 600; letter-spacing: .03em;
  }
  .badge-green { background: var(--green-dim); color: var(--green); border: 1px solid rgba(63, 185, 80, 0.3); }
  .badge-yellow { background: rgba(210, 153, 34, 0.15); color: var(--yellow); border: 1px solid rgba(210, 153, 34, 0.3); }
  .badge-purple { background: rgba(188, 140, 255, 0.15); color: var(--purple); border: 1px solid rgba(188, 140, 255, 0.3); }
  .badge-gray { background: var(--tag-bg); color: var(--muted); border: 1px solid var(--border); }
  .section { margin-bottom: 36px; }
  .section-title {
    font-size: 12px; font-weight: 700; color: var(--muted);
    text-transform: uppercase; letter-spacing: .08em;
    margin-bottom: 14px; padding-bottom: 8px;
    border-bottom: 1px solid var(--border);
    display: flex; align-items: center; justify-content: space-between;
  }
  .card-stack { display: flex; flex-direction: column; gap: 16px; }
  .mw-card {
    background: var(--surface); border: 1px solid var(--border);
    border-radius: 8px; padding: 18px 20px; transition: border-color .15s;
  }
  .mw-card:hover { border-color: var(--accent); }
  .mw-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 6px; }
  .mw-name { font-size: 15px; font-weight: 700; font-family: ui-monospace, SFMono-Regular, SF Mono, Menlo, monospace; color: var(--accent); }
  .mw-desc { color: var(--text); font-size: 13.5px; margin-bottom: 14px; }
  .mw-sub-title { font-size: 11px; font-weight: 700; color: var(--muted); text-transform: uppercase; letter-spacing: .06em; margin-top: 12px; margin-bottom: 6px; }
  .mw-list { list-style: none; padding: 0; margin: 0; }
  .mw-list-item { font-size: 13px; color: var(--text); margin-bottom: 4px; display: flex; align-items: baseline; gap: 6px; }
  .mw-list-bullet { color: var(--accent); font-weight: bold; }
  .mw-prop-key { font-family: ui-monospace, SFMono-Regular, SF Mono, Menlo, monospace; font-weight: 600; color: var(--accent); }
  .mw-prop-val { font-family: ui-monospace, SFMono-Regular, SF Mono, Menlo, monospace; font-weight: 600; color: var(--text); background: var(--tag-bg); padding: 1px 5px; border-radius: 4px; border: 1px solid var(--border); }
  .mw-prop-desc { color: var(--muted); font-size: 12.5px; }

  /* 2-Column Split View Grid */
  .split-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 20px; width: 100%; }
  @media (max-width: 860px) {
    .split-grid { grid-template-columns: 1fr; }
  }
  .split-panel {
    background: var(--surface); border: 1px solid var(--border);
    border-radius: 8px; padding: 18px; display: flex; flex-direction: column;
    min-width: 0; width: 100%; overflow-x: auto;
  }
  .split-panel-title {
    font-size: 11px; font-weight: 700; color: var(--muted);
    text-transform: uppercase; letter-spacing: .08em; margin-bottom: 12px;
    padding-bottom: 6px; border-bottom: 1px solid var(--border);
  }

  .fn-card {
    background: var(--surface); border: 1px solid var(--border);
    border-radius: 8px; padding: 18px 22px; margin-bottom: 14px;
    transition: border-color .15s, background .15s;
  }
  .fn-card:hover { border-color: var(--accent); background: var(--surface-hover); }
  .fn-card-name { font-size: 16px; font-weight: 700; font-family: ui-monospace, SFMono-Regular, SF Mono, Menlo, monospace; color: var(--text); }
  .fn-card-desc { color: var(--muted); margin-top: 4px; font-size: 13.5px; }
  .fn-card-meta { display: flex; gap: 8px; margin-top: 12px; flex-wrap: wrap; }
  .code-block-wrap { position: relative; margin-top: 4px; flex: 1; min-width: 0; width: 100%; overflow-x: auto; }
  .code-block {
    background: var(--code-bg); border: 1px solid var(--border);
    border-radius: 8px; padding: 14px; overflow-x: auto;
    font-family: ui-monospace, SFMono-Regular, SF Mono, Menlo, monospace;
    font-size: 12px; line-height: 1.6; color: #adbac7;
    white-space: pre; height: 100%; max-width: 100%;
  }
  .copy-btn {
    position: absolute; top: 8px; right: 8px;
    background: var(--tag-bg); border: 1px solid var(--border);
    color: var(--muted); border-radius: 6px;
    padding: 3px 8px; font-size: 11px; cursor: pointer;
    transition: all .15s;
  }
  .copy-btn:hover { background: var(--border); color: var(--text); }
  .copy-btn.copied { color: var(--green); border-color: var(--green); }
  .tag {
    background: var(--tag-bg); border: 1px solid var(--border);
    color: var(--muted); border-radius: 4px;
    padding: 2px 8px; font-size: 11px;
  }
  .info-row { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; margin-bottom: 16px; }
  .search-bar {
    width: 100%; background: var(--surface); border: 1px solid var(--border);
    border-radius: 8px; padding: 12px 16px; color: var(--text);
    font-size: 14px; margin-bottom: 24px; outline: none; transition: border-color .15s;
  }
  .search-bar:focus { border-color: var(--accent); }
  .ns-header { font-size: 18px; font-weight: 700; margin: 32px 0 14px; letter-spacing: -0.01em; }
  .ns-header:first-child { margin-top: 0; }
  .empty { color: var(--muted); font-size: 14px; padding: 40px 0; text-align: center; }
  .header-table {
    width: 100%; border-collapse: collapse; background: var(--surface);
    border: 1px solid var(--border); border-radius: 8px; overflow: hidden;
  }
  .header-table th {
    background: var(--code-bg); padding: 10px 14px; text-align: left;
    font-size: 11px; font-weight: 700; color: var(--muted); text-transform: uppercase;
    letter-spacing: .06em; border-bottom: 1px solid var(--border);
  }
  .header-table td {
    padding: 10px 14px; font-size: 13px; border-bottom: 1px solid var(--border);
  }
  .header-table tr:last-child td { border-bottom: none; }
  .header-key { font-family: ui-monospace, SFMono-Regular, SF Mono, Menlo, monospace; font-weight: 600; color: var(--accent); }
`

const copyScript = `
<script>
function copyCode(btn) {
  const block = btn.previousElementSibling;
  navigator.clipboard.writeText(block.textContent).then(() => {
    btn.textContent = 'Copied!';
    btn.classList.add('copied');
    setTimeout(() => { btn.textContent = 'Copy'; btn.classList.remove('copied'); }, 2000);
  });
}
</script>
`

func normalizeDocsEndpoint(endpoint string) string {
	cleaned := strings.TrimSpace(endpoint)
	if cleaned == "" {
		cleaned = "/docs"
	}
	if !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	if len(cleaned) > 1 && strings.HasSuffix(cleaned, "/") {
		cleaned = cleaned[:len(cleaned)-1]
	}
	return cleaned
}

func (app *CoreRPC) handleDocsRequest(reqCtx *fasthttp.RequestCtx) {
	normDocs := normalizeDocsEndpoint(app.config.DocsPath)
	cleanRpc := strings.TrimSpace(app.config.RpcPath)
	if !strings.HasPrefix(cleanRpc, "/") {
		cleanRpc = "/" + cleanRpc
	}

	proto := "http"
	if string(reqCtx.Request.Header.Peek("X-Forwarded-Proto")) == "https" || reqCtx.IsTLS() {
		proto = "https"
	}
	serverUrl := fmt.Sprintf("%s://%s", proto, string(reqCtx.Host()))

	rawPath := string(reqCtx.Path())
	cleanPath := strings.TrimRight(strings.Split(rawPath, "?")[0], "/")
	if cleanPath == "" {
		cleanPath = normDocs
	}

	var relativeSubPath string
	if cleanPath == normDocs {
		relativeSubPath = ""
	} else if strings.HasPrefix(cleanPath, normDocs+"/") {
		relativeSubPath = strings.TrimPrefix(cleanPath, normDocs+"/")
	} else {
		relativeSubPath = strings.TrimPrefix(cleanPath, "/")
	}

	app.mu.RLock()
	functions := app.Functions()
	middlewares := app.Middlewares()
	app.mu.RUnlock()

	reqCtx.SetContentType("text/html; charset=utf-8")

	if relativeSubPath == "" {
		reqCtx.SetBodyString(renderDocsIndex(normDocs, cleanRpc, serverUrl, functions, middlewares))
		return
	}

	parts := strings.Split(relativeSubPath, "/")

	// 1. Central Middleware documentation view: /docs/middlewares/:name
	if len(parts) == 2 && parts[0] == "middlewares" {
		mwName := parts[1]
		if mw, ok := middlewares[mwName]; ok {
			reqCtx.SetBodyString(renderMiddlewareDoc(normDocs, mw, functions, middlewares))
			return
		}
		reqCtx.SetStatusCode(fasthttp.StatusNotFound)
		reqCtx.SetBodyString(renderDocsNotFound(normDocs))
		return
	}

	// 2. Legacy /docs/fn/:key
	if len(parts) == 2 && parts[0] == "fn" {
		fnKey := parts[1]
		if fn, ok := functions[fnKey]; ok {
			reqCtx.SetBodyString(renderFunctionDocs(normDocs, cleanRpc, serverUrl, fn, middlewares, functions))
			return
		}
	}

	// 3. Standard /docs/{namespace}/{functionName} (supporting deep namespaces like /docs/admin/products/list)
	if len(parts) >= 2 {
		nsParts := parts[:len(parts)-1]
		fnName := parts[len(parts)-1]
		ns := strings.Join(nsParts, ".")
		targetKey := ns + "." + fnName

		if fn, ok := functions[targetKey]; ok {
			reqCtx.SetBodyString(renderFunctionDocs(normDocs, cleanRpc, serverUrl, fn, middlewares, functions))
			return
		}
	}

	reqCtx.SetStatusCode(fasthttp.StatusNotFound)
	reqCtx.SetBodyString(renderDocsNotFound(normDocs))
}

func htmlShell(title, bodyContent, sidebarHtml, docsEndpoint string) string {
	normDocs := normalizeDocsEndpoint(docsEndpoint)
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s</title>
<style>%s</style>
</head>
<body>
<div class="layout">
  <nav class="sidebar">
    <div class="sidebar-header">
      <a href="%s" style="text-decoration:none">
        <div class="sidebar-logo">
          <span>CellsWeb RPC - <em style="color:#00ADD8;font-style:italic;font-weight:700;">Go</em></span>
          <span class="sidebar-logo-badge">DOCS</span>
        </div>
      </a>
    </div>
    %s
  </nav>
  <main class="main">
    %s
  </main>
</div>
%s
</body>
</html>`, html.EscapeString(title), docsCSS, normDocs, sidebarHtml, bodyContent, copyScript)
}

func codeBlock(content string) string {
	return fmt.Sprintf(`<div class="code-block-wrap">
    <pre class="code-block">%s</pre>
    <button class="copy-btn" onclick="copyCode(this)">Copy</button>
  </div>`, html.EscapeString(content))
}

func buildSidebar(functions map[string]*FunctionDefinition, middlewares map[string]*RegisteredMiddleware, activeKey, docsEndpoint string) string {
	normDocs := normalizeDocsEndpoint(docsEndpoint)

	// Group functions by namespace
	grouped := make(map[string][]*FunctionDefinition)
	for _, fn := range functions {
		grouped[fn.Namespace] = append(grouped[fn.Namespace], fn)
	}

	sortedNs := make([]string, 0, len(grouped))
	for ns := range grouped {
		sortedNs = append(sortedNs, ns)
	}
	sort.Strings(sortedNs)

	var out strings.Builder
	for _, ns := range sortedNs {
		out.WriteString(fmt.Sprintf(`<div class="sidebar-ns">%s</div>`, html.EscapeString(ns)))
		fns := grouped[ns]
		sort.Slice(fns, func(i, j int) bool {
			return fns[i].FunctionName < fns[j].FunctionName
		})
		for _, fn := range fns {
			isActive := fn.Key == activeKey
			activeClass := ""
			if isActive {
				activeClass = " active"
			}
			isJson := fn.ContentType == "application/json"
			badgeText := "JSON"
			if !isJson {
				badgeText = "FORM"
			}
			nsUrl := strings.ReplaceAll(fn.Namespace, ".", "/")
			out.WriteString(fmt.Sprintf(`
        <a class="sidebar-fn%s" href="%s/%s/%s">
          <span>%s</span>
          <span class="sidebar-fn-badge">%s</span>
        </a>`,
				activeClass,
				normDocs,
				html.EscapeString(nsUrl),
				html.EscapeString(fn.FunctionName),
				html.EscapeString(fn.FunctionName),
				badgeText,
			))
		}
	}

	// Middlewares in sidebar
	if len(middlewares) > 0 {
		mwList := make([]*RegisteredMiddleware, 0, len(middlewares))
		for _, mw := range middlewares {
			mwList = append(mwList, mw)
		}
		sort.Slice(mwList, func(i, j int) bool {
			return mwList[i].Metadata.Name < mwList[j].Metadata.Name
		})

		out.WriteString(`<div class="sidebar-ns" style="margin-top:20px;border-top:1px solid var(--border);padding-top:12px">Middlewares</div>`)
		for _, mw := range mwList {
			isActive := activeKey == "mw:"+mw.Metadata.Name
			activeClass := ""
			if isActive {
				activeClass = " active"
			}
			out.WriteString(fmt.Sprintf(`
        <a class="sidebar-fn%s" href="%s/middlewares/%s">
          <span>⚡ %s</span>
          <span class="sidebar-fn-badge">MW</span>
        </a>`,
				activeClass,
				normDocs,
				html.EscapeString(mw.Metadata.Name),
				html.EscapeString(mw.Metadata.Name),
			))
		}
	}

	if out.Len() == 0 {
		return `<div class="sidebar-fn" style="color:var(--muted)">No functions loaded</div>`
	}
	return out.String()
}

func formatSchemaType(rule *schema.SchemaRule) string {
	if rule == nil {
		return "any"
	}
	if rule.Type == schema.TypeFile || rule.Format == "binary" {
		return "File / Binary"
	}
	if rule.Type == schema.TypeArray {
		if rule.Items != nil {
			if rule.Items.Type == schema.TypeFile || rule.Items.Format == "binary" {
				return "File[] / Binary[]"
			}
			return formatSchemaType(rule.Items) + "[]"
		}
		return "any[]"
	}
	if rule.Type == schema.TypeString {
		return "string"
	}
	if rule.Type == schema.TypeNumber {
		return "number"
	}
	if rule.Type == schema.TypeInteger {
		return "integer"
	}
	if rule.Type == schema.TypeBoolean {
		return "boolean"
	}
	if rule.Type == schema.TypeObject {
		return "object"
	}
	return string(rule.Type)
}

func isPropertyRequired(parent *schema.SchemaRule, key string, prop *schema.SchemaRule) bool {
	if prop.Optional {
		return false
	}
	if parent != nil && len(parent.Required) > 0 {
		for _, req := range parent.Required {
			if req == key {
				return true
			}
		}
		return false
	}
	return !prop.Optional
}

func renderSchemaFieldsList(rule *schema.SchemaRule, indent int) string {
	if rule == nil {
		return `<div style="color:var(--muted);font-size:12.5px">• Any payload allowed</div>`
	}

	if rule.Type == schema.TypeObject && (rule.Properties == nil || len(rule.Properties) == 0) {
		return `<div style="color:var(--muted);font-size:12.5px">• No fields specified (empty object)</div>`
	}

	props := rule.Properties
	if props == nil {
		if rule.Type == schema.TypeArray && rule.Items != nil {
			return renderSchemaFieldsList(rule.Items, indent)
		}
		return `<div style="color:var(--muted);font-size:12.5px">• Any payload allowed</div>`
	}

	propKeys := make([]string, 0, len(props))
	for k := range props {
		propKeys = append(propKeys, k)
	}
	sort.Strings(propKeys)

	var items strings.Builder
	for _, key := range propKeys {
		pSchema := props[key]
		isRequired := isPropertyRequired(rule, key, pSchema)
		typeStr := formatSchemaType(pSchema)
		desc := pSchema.Description

		nestedHtml := ""
		if pSchema.Type == schema.TypeObject && len(pSchema.Properties) > 0 {
			nestedHtml = fmt.Sprintf(`<div style="margin-left:14px;margin-top:4px">%s</div>`, renderSchemaFieldsList(pSchema, indent+1))
		} else if pSchema.Type == schema.TypeArray && pSchema.Items != nil && len(pSchema.Items.Properties) > 0 {
			nestedHtml = fmt.Sprintf(`<div style="margin-left:14px;margin-top:4px">%s</div>`, renderSchemaFieldsList(pSchema.Items, indent+1))
		}

		statusHtml := "optional"
		if isRequired {
			statusHtml = `<strong style="color:var(--accent)">required</strong>`
		}

		descHtml := ""
		if desc != "" {
			descHtml = fmt.Sprintf(` <span class="mw-prop-desc">— %s</span>`, html.EscapeString(desc))
		}

		items.WriteString(fmt.Sprintf(`
      <li class="mw-list-item" style="display:block;margin-bottom:6px">
        <div style="display:flex;align-items:baseline;gap:6px">
          <span class="mw-list-bullet">•</span>
          <span class="mw-prop-key">%s</span>
          <span style="font-size:11px;color:var(--muted)">(%s, %s)</span>
          %s
        </div>
        %s
      </li>`,
			html.EscapeString(key),
			html.EscapeString(typeStr),
			statusHtml,
			descHtml,
			nestedHtml,
		))
	}

	return fmt.Sprintf(`<ul class="mw-list">%s</ul>`, items.String())
}

func render2ColumnSchemaSection(sectionTitle string, rule *schema.SchemaRule, contentType string) string {
	if rule == nil {
		return fmt.Sprintf(`
      <div class="section">
        <div class="section-title">%s</div>
        <div style="color:var(--muted);font-size:13px">No schema defined.</div>
      </div>`, html.EscapeString(sectionTitle))
	}

	isMultipart := contentType == "multipart/form-data"
	leftLabel := "Required Data Fields"
	if isMultipart {
		leftLabel = "Form Fields & File Uploads"
	}

	fieldsListHtml := renderSchemaFieldsList(rule, 0)
	rawJsonSchema, _ := schema.ToJSONString(rule)

	return fmt.Sprintf(`
    <div class="section">
      <div class="section-title">%s</div>
      <div class="split-grid">
        <div class="split-panel">
          <div class="split-panel-title">%s</div>
          %s
        </div>
        <div class="split-panel">
          <div class="split-panel-title">JSON Schema</div>
          %s
        </div>
      </div>
    </div>`,
		html.EscapeString(sectionTitle),
		html.EscapeString(leftLabel),
		fieldsListHtml,
		codeBlock(rawJsonSchema),
	)
}

func renderMiddlewareCardContent(
	mwName string,
	rawDescription string,
	optsConfigured map[string]any,
	centralMw *RegisteredMiddleware,
	isGlobalView bool,
	docsEndpoint string,
) string {
	normDocs := normalizeDocsEndpoint(docsEndpoint)

	// 1. Properties & Controls List
	var propsListHtml string
	if centralMw != nil && centralMw.Metadata.Options != nil && len(centralMw.Metadata.Options.Properties) > 0 {
		optProps := centralMw.Metadata.Options.Properties
		propKeys := make([]string, 0, len(optProps))
		for k := range optProps {
			propKeys = append(propKeys, k)
		}
		sort.Strings(propKeys)

		var items strings.Builder
		for _, key := range propKeys {
			pSchema := optProps[key]
			desc := pSchema.Description

			var defaultValStr, configuredValStr string
			if pSchema.Default != nil {
				b, _ := json.Marshal(pSchema.Default)
				defaultValStr = string(b)
			}
			if optsConfigured != nil {
				if val, exists := optsConfigured[key]; exists {
					b, _ := json.Marshal(val)
					configuredValStr = string(b)
				}
			}

			displayVal := configuredValStr
			isDefaultTag := false
			if displayVal == "" {
				if defaultValStr != "" {
					displayVal = defaultValStr
					isDefaultTag = isGlobalView
				} else {
					displayVal = "-"
				}
			}

			tagHtml := ""
			if isDefaultTag {
				tagHtml = ` <span style="font-size:10px;color:var(--muted)">(default)</span>`
			}
			descHtml := ""
			if desc != "" {
				descHtml = fmt.Sprintf(` <span class="mw-prop-desc">— %s</span>`, html.EscapeString(desc))
			}

			items.WriteString(fmt.Sprintf(`
        <li class="mw-list-item">
          <span class="mw-list-bullet">•</span>
          <span class="mw-prop-key">%s:</span>
          <span class="mw-prop-val">%s%s</span>
          %s
        </li>`,
				html.EscapeString(key),
				html.EscapeString(displayVal),
				tagHtml,
				descHtml,
			))
		}
		propsListHtml = fmt.Sprintf(`<ul class="mw-list">%s</ul>`, items.String())
	} else if len(optsConfigured) > 0 {
		keys := make([]string, 0, len(optsConfigured))
		for k := range optsConfigured {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var items strings.Builder
		for _, k := range keys {
			b, _ := json.Marshal(optsConfigured[k])
			items.WriteString(fmt.Sprintf(`
        <li class="mw-list-item">
          <span class="mw-list-bullet">•</span>
          <span class="mw-prop-key">%s:</span>
          <span class="mw-prop-val">%s</span>
        </li>`, html.EscapeString(k), html.EscapeString(string(b))))
		}
		propsListHtml = fmt.Sprintf(`<ul class="mw-list">%s</ul>`, items.String())
	} else {
		propsListHtml = `<div style="color:var(--muted);font-size:12.5px;margin-top:4px">• None (No configurable options)</div>`
	}

	// 2. Required Headers List
	var headersListHtml string
	if centralMw != nil && centralMw.Metadata.Headers != nil && len(centralMw.Metadata.Headers.Properties) > 0 {
		headerProps := centralMw.Metadata.Headers.Properties
		headerKeys := make([]string, 0, len(headerProps))
		for k := range headerProps {
			headerKeys = append(headerKeys, k)
		}
		sort.Strings(headerKeys)

		var items strings.Builder
		for _, key := range headerKeys {
			hSchema := headerProps[key]
			desc := hSchema.Description
			if desc == "" {
				desc = "HTTP Header value"
			}
			items.WriteString(fmt.Sprintf(`
        <li class="mw-list-item">
          <span class="mw-list-bullet">•</span>
          <span class="mw-prop-key">%s</span>
          <span class="mw-prop-desc">— %s</span>
        </li>`, html.EscapeString(key), html.EscapeString(desc)))
		}
		headersListHtml = fmt.Sprintf(`<ul class="mw-list">%s</ul>`, items.String())
	} else {
		headersListHtml = `<div style="color:var(--muted);font-size:12.5px;margin-top:4px">• None (No headers required)</div>`
	}

	return fmt.Sprintf(`
    <div class="mw-card">
      <div class="mw-header">
        <a href="%s/middlewares/%s" style="text-decoration:none"><div class="mw-name">⚡ %s</div></a>
      </div>
      <div class="mw-desc">%s</div>
      
      <div class="mw-sub-title">Properties & Controls:</div>
      %s

      <div class="mw-sub-title">Required Headers:</div>
      %s
    </div>`,
		normDocs,
		html.EscapeString(mwName),
		html.EscapeString(mwName),
		html.EscapeString(rawDescription),
		propsListHtml,
		headersListHtml,
	)
}

func generateSampleData(rule *schema.SchemaRule) any {
	if rule == nil {
		return map[string]any{}
	}
	if rule.Default != nil {
		return rule.Default
	}

	switch rule.Type {
	case schema.TypeString:
		if rule.Format == "email" {
			return "user@example.com"
		}
		if rule.Format == "binary" {
			return "(File)"
		}
		return "example"
	case schema.TypeFile:
		return "(File)"
	case schema.TypeNumber:
		if rule.Minimum != nil {
			return *rule.Minimum
		}
		return 1.5
	case schema.TypeInteger:
		if rule.Minimum != nil {
			return int(*rule.Minimum)
		}
		return 1
	case schema.TypeBoolean:
		return true
	case schema.TypeArray:
		if rule.Items != nil {
			return []any{generateSampleData(rule.Items)}
		}
		return []any{"example"}
	case schema.TypeObject:
		res := make(map[string]any)
		if rule.Properties != nil {
			keys := make([]string, 0, len(rule.Properties))
			for k := range rule.Properties {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				res[k] = generateSampleData(rule.Properties[k])
			}
		}
		return res
	default:
		return "example"
	}
}

func renderDocsIndex(
	docsEndpoint, rpcEndpoint, serverUrl string,
	functions map[string]*FunctionDefinition,
	middlewares map[string]*RegisteredMiddleware,
) string {
	normDocs := normalizeDocsEndpoint(docsEndpoint)
	cleanServer := strings.TrimRight(serverUrl, "/")
	cleanRpc := strings.TrimSpace(rpcEndpoint)
	if !strings.HasPrefix(cleanRpc, "/") {
		cleanRpc = "/" + cleanRpc
	}

	grouped := make(map[string][]*FunctionDefinition)
	for _, fn := range functions {
		grouped[fn.Namespace] = append(grouped[fn.Namespace], fn)
	}
	sortedNs := make([]string, 0, len(grouped))
	for ns := range grouped {
		sortedNs = append(sortedNs, ns)
	}
	sort.Strings(sortedNs)

	endpointBadge := ""
	if cleanServer != "" {
		endpointBadge = fmt.Sprintf(
			`<div style="margin-top:8px"><span class="badge badge-purple" style="font-family:ui-monospace, SFMono-Regular, monospace;font-size:12px;padding:4px 10px;">POST %s%s</span></div>`,
			html.EscapeString(cleanServer),
			html.EscapeString(cleanRpc),
		)
	}

	var body strings.Builder
	body.WriteString(fmt.Sprintf(`
    <div class="page-header">
      <div class="page-title">API Documentation</div>
      <div class="page-sub">%d functions across %d namespaces</div>
      %s
    </div>
    <input class="search-bar" type="search" placeholder="Search functions, namespaces, or middlewares..." oninput="filterFns(this.value)" id="search-input">
    <div id="fn-list">`,
		len(functions), len(sortedNs), endpointBadge))

	if len(sortedNs) == 0 {
		body.WriteString(`<div class="empty">No functions registered yet.<br>Add functions to your RPC application</div>`)
	}

	for _, ns := range sortedNs {
		body.WriteString(fmt.Sprintf(`<div class="ns-header" data-ns="%s">%s</div>`, html.EscapeString(ns), html.EscapeString(ns)))
		fns := grouped[ns]
		sort.Slice(fns, func(i, j int) bool {
			return fns[i].FunctionName < fns[j].FunctionName
		})
		for _, fn := range fns {
			tagsHtml := ""
			for _, t := range fn.Metadata.Tags {
				tagsHtml += fmt.Sprintf(`<span class="tag">%s</span> `, html.EscapeString(t))
			}

			mwNames := make([]string, 0, len(fn.Middlewares))
			for _, m := range fn.Middlewares {
				mwNames = append(mwNames, m.Name)
			}
			mwSubtitle := "No middleware"
			if len(mwNames) > 0 {
				mwSubtitle = "Middlewares: " + strings.Join(mwNames, ", ")
			}

			publicBadge := `<span class="badge badge-gray">PRIVATE</span>`
			if fn.Metadata.IsPublic {
				publicBadge = `<span class="badge badge-green">PUBLIC</span>`
			}
			depBadge := ""
			if fn.Metadata.Deprecated {
				depBadge = `<span class="badge badge-yellow">DEPRECATED</span>`
			}

			descHtml := ""
			if fn.Metadata.Description != "" {
				descHtml = fmt.Sprintf(`<div class="fn-card-desc">%s</div>`, html.EscapeString(fn.Metadata.Description))
			}

			nsUrl := strings.ReplaceAll(fn.Namespace, ".", "/")
			body.WriteString(fmt.Sprintf(`
        <a href="%s/%s/%s" style="text-decoration:none;display:block;">
          <div class="fn-card" data-name="%s" data-ns="%s">
            <div class="fn-card-name">%s</div>
            %s
            <div class="fn-card-meta">
              %s
              %s
              <span class="badge badge-purple">%s</span>
              <span class="badge badge-gray">%s</span>
              %s
            </div>
          </div>
        </a>`,
				normDocs,
				html.EscapeString(nsUrl),
				html.EscapeString(fn.FunctionName),
				html.EscapeString(fn.FunctionName),
				html.EscapeString(ns),
				html.EscapeString(fn.FunctionName),
				descHtml,
				publicBadge,
				depBadge,
				html.EscapeString(fn.ContentType),
				html.EscapeString(mwSubtitle),
				tagsHtml,
			))
		}
	}
	body.WriteString(`</div>`)

	// Central Middlewares Catalog Section on Index Page
	if len(middlewares) > 0 {
		mwList := make([]*RegisteredMiddleware, 0, len(middlewares))
		for _, mw := range middlewares {
			mwList = append(mwList, mw)
		}
		sort.Slice(mwList, func(i, j int) bool {
			return mwList[i].Metadata.Name < mwList[j].Metadata.Name
		})

		body.WriteString(fmt.Sprintf(`
      <div style="margin-top:48px;border-top:1px solid var(--border);padding-top:32px">
        <div style="font-size:20px;font-weight:700;margin-bottom:8px">⚡ Central Middlewares Catalog (%d)</div>
        <p style="color:var(--muted);font-size:13.5px;margin-bottom:20px">
          Middlewares process incoming requests, enforce security policies, validate headers, and inject context before functions run.
        </p>
        <div class="card-stack">`, len(mwList)))

		for _, mw := range mwList {
			rawDesc := mw.Metadata.Description
			if rawDesc == "" {
				rawDesc = fmt.Sprintf("Custom %s middleware", mw.Metadata.Name)
			}
			body.WriteString(renderMiddlewareCardContent(mw.Metadata.Name, rawDesc, nil, mw, true, normDocs))
		}
		body.WriteString(`</div></div>`)
	}

	// Error Reference Guide
	errorExample := `{
  "status": "error",
  "code": "validation_error",
  "message": "Input validation failed: email — Invalid email address",
  "meta": {
    "durationMs": 1.84,
    "requestId": "clx123abc456"
  }
}`

	errorCodes := []struct {
		code string
		when string
	}{
		{code: "invalid_content_type", when: "Content-Type is missing or not application/json or multipart/form-data"},
		{code: "invalid_body", when: "Missing, malformed, or empty request body (namespace, functionName, data)"},
		{code: "function_not_found", when: "The requested function is not registered in the RPC server"},
		{code: "validation_error", when: "Input data failed the function's input schema validation contract"},
		{code: "unauthorized", when: "Auth middleware rejected credentials or authorization token"},
		{code: "forbidden", when: "User is authenticated but lacks required role or permission"},
		{code: "rate_limited", when: "Caller exceeded configured rate limits (windowSeconds / maxRequests)"},
		{code: "schema_mismatch", when: "Function output failed outputSchema contract — server implementation bug"},
		{code: "payload_too_large", when: "Request body or uploaded file size exceeds maximum limits"},
		{code: "timeout", when: "Function execution exceeded configured timeout threshold"},
		{code: "internal_error", when: "Internal unhandled error or missing middleware registration"},
	}

	var errorRows strings.Builder
	for _, ec := range errorCodes {
		errorRows.WriteString(fmt.Sprintf(`
      <tr>
        <td style="padding:8px 12px;font-family:ui-monospace, SFMono-Regular, monospace;font-size:13px;color:var(--accent);white-space:nowrap">%s</td>
        <td style="padding:8px 12px;font-size:13px;color:var(--muted)">%s</td>
      </tr>`, html.EscapeString(ec.code), html.EscapeString(ec.when)))
	}

	body.WriteString(fmt.Sprintf(`
    <div class="section" style="margin-top:48px;border-top:1px solid var(--border);padding-top:32px">
      <div style="font-size:20px;font-weight:700;margin-bottom:8px">⚠ Error Reference</div>
      <p style="color:var(--muted);font-size:13.5px;margin-bottom:20px">
        The HTTP status is always <strong style="color:var(--text)">200</strong>.
        Error information lives entirely in the JSON response body. Check <code>status === "error"</code>
        and branch on <code>code</code> to handle specific failure modes.
      </p>

      <div class="section-title">Error Response Example</div>
      %s

      <div class="section-title" style="margin-top:24px">Error Code Enum</div>
      <div style="border:1px solid var(--border);border-radius:8px;overflow:hidden">
        <table style="width:100%%;border-collapse:collapse">
          <thead>
            <tr style="background:var(--code-bg)">
              <th style="padding:8px 12px;text-align:left;font-size:11px;font-weight:600;color:var(--muted);text-transform:uppercase;letter-spacing:.06em;border-bottom:1px solid var(--border)">code</th>
              <th style="padding:8px 12px;text-align:left;font-size:11px;font-weight:600;color:var(--muted);text-transform:uppercase;letter-spacing:.06em;border-bottom:1px solid var(--border)">When it fires</th>
            </tr>
          </thead>
          <tbody>
            %s
          </tbody>
        </table>
      </div>
    </div>`, codeBlock(errorExample), errorRows.String()))

	// Client-side search JS
	body.WriteString(`
    <script>
    function filterFns(query) {
      if (!query) {
        document.querySelectorAll('.fn-card, .mw-card').forEach(c => (c.parentElement || c).style.display = '');
        document.querySelectorAll('.ns-header').forEach(h => h.style.display = '');
        return;
      }
      
      let regex;
      try {
        regex = new RegExp(query, 'i');
      } catch (e) {
        regex = new RegExp(query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'i');
      }

      const fuzzyPattern = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&').split('').join('.*?');
      const fuzzyRegex = new RegExp(fuzzyPattern, 'i');

      const nsMatched = new Set();
      const fnMatchedNs = new Set();
      const cards = Array.from(document.querySelectorAll('.fn-card'));

      cards.forEach(card => {
        const name = card.dataset.name || '';
        const ns = card.dataset.ns || '';
        const isNsMatch = regex.test(ns) || fuzzyRegex.test(ns);
        const isFnMatch = regex.test(name) || fuzzyRegex.test(name);
        if (isNsMatch) nsMatched.add(ns);
        if (isFnMatch) fnMatchedNs.add(ns);
      });

      cards.forEach(card => {
        const name = card.dataset.name || '';
        const ns = card.dataset.ns || '';
        const isFnMatch = regex.test(name) || fuzzyRegex.test(name);
        let show = false;
        if (fnMatchedNs.has(ns)) {
          if (isFnMatch) show = true;
        } else if (nsMatched.has(ns)) {
          show = true;
        }
        card.parentElement.style.display = show ? '' : 'none';
      });

      document.querySelectorAll('.ns-header').forEach(header => {
        const ns = header.dataset.ns || '';
        const hasVisibleChild = cards.some(c => c.dataset.ns === ns && c.parentElement.style.display !== 'none');
        header.style.display = hasVisibleChild ? '' : 'none';
      });
    }
    </script>
  `)

	return htmlShell("API Documentation", body.String(), buildSidebar(functions, middlewares, "", normDocs), normDocs)
}

func renderMiddlewareDoc(
	docsEndpoint string,
	mw *RegisteredMiddleware,
	functions map[string]*FunctionDefinition,
	middlewares map[string]*RegisteredMiddleware,
) string {
	normDocs := normalizeDocsEndpoint(docsEndpoint)
	rawDesc := mw.Metadata.Description
	if rawDesc == "" {
		rawDesc = fmt.Sprintf("Custom %s middleware", mw.Metadata.Name)
	}

	var body strings.Builder
	body.WriteString(fmt.Sprintf(`
    <div class="page-header">
      <div class="page-breadcrumb">
        <a href="%s">← All functions & middlewares</a>
        <span>/</span>
        <span>middlewares</span>
      </div>
      <div class="page-title">⚡ %s</div>
      <div class="page-sub">%s</div>
      <div class="info-row" style="margin-top:14px">
        <span class="badge badge-purple">CENTRAL MIDDLEWARE</span>
      </div>
    </div>`,
		normDocs,
		html.EscapeString(mw.Metadata.Name),
		html.EscapeString(rawDesc),
	))

	body.WriteString(fmt.Sprintf(`
    <div class="section">
      <div class="section-title">Middleware Specification</div>
      %s
    </div>`, renderMiddlewareCardContent(mw.Metadata.Name, rawDesc, nil, mw, true, normDocs)))

	if mw.Metadata.Options != nil {
		optsJson, _ := schema.ToJSONString(mw.Metadata.Options)
		body.WriteString(fmt.Sprintf(`
      <div class="section">
        <div class="section-title">Options JSON Schema</div>
        %s
      </div>`, codeBlock(optsJson)))
	}

	if mw.Metadata.Headers != nil {
		headersJson, _ := schema.ToJSONString(mw.Metadata.Headers)
		body.WriteString(fmt.Sprintf(`
      <div class="section">
        <div class="section-title">Required Headers JSON Schema</div>
        %s
      </div>`, codeBlock(headersJson)))
	}

	return htmlShell(fmt.Sprintf("⚡ %s — Middleware Docs", mw.Metadata.Name), body.String(), buildSidebar(functions, middlewares, "mw:"+mw.Metadata.Name, normDocs), normDocs)
}

func renderFunctionDocs(
	docsEndpoint, rpcEndpoint, serverUrl string,
	fn *FunctionDefinition,
	allMws map[string]*RegisteredMiddleware,
	allFunctions map[string]*FunctionDefinition,
) string {
	normDocs := normalizeDocsEndpoint(docsEndpoint)
	cleanServer := strings.TrimRight(serverUrl, "/")
	cleanRpc := strings.TrimSpace(rpcEndpoint)
	if !strings.HasPrefix(cleanRpc, "/") {
		cleanRpc = "/" + cleanRpc
	}
	fullRpcUrl := fmt.Sprintf("%s%s", cleanServer, cleanRpc)

	inputSample := generateSampleData(fn.InputSchema)
	outputSample := generateSampleData(fn.OutputSchema)

	requestExample := map[string]any{
		"namespace":    fn.Namespace,
		"functionName": fn.FunctionName,
		"data":         inputSample,
	}
	reqBytes, _ := json.MarshalIndent(requestExample, "", "  ")

	responseExample := map[string]any{
		"status": "ok",
		"data":   outputSample,
		"meta": map[string]any{
			"requestId":  "clx123abc456",
			"durationMs": 1.42,
		},
	}
	resBytes, _ := json.MarshalIndent(responseExample, "", "  ")

	tagsHtml := ""
	for _, t := range fn.Metadata.Tags {
		tagsHtml += fmt.Sprintf(`<span class="tag">%s</span> `, html.EscapeString(t))
	}

	// Middlewares and Headers Inspection
	middlewareCards := make([]string, 0, len(fn.Middlewares))
	type headerReqInfo struct {
		description string
		requiredBy  []string
	}
	requiredHeadersMap := make(map[string]*headerReqInfo)

	for _, mwItem := range fn.Middlewares {
		centralMw := allMws[mwItem.Name]
		rawDesc := "Custom middleware"
		if centralMw != nil && centralMw.Metadata.Description != "" {
			rawDesc = centralMw.Metadata.Description
		}
		middlewareCards = append(middlewareCards, renderMiddlewareCardContent(mwItem.Name, rawDesc, mwItem.Options, centralMw, false, normDocs))

		if centralMw != nil && centralMw.Metadata.Headers != nil && len(centralMw.Metadata.Headers.Properties) > 0 {
			for hKey, hRule := range centralMw.Metadata.Headers.Properties {
				keyLower := strings.ToLower(hKey)
				info, exists := requiredHeadersMap[keyLower]
				if !exists {
					info = &headerReqInfo{}
					requiredHeadersMap[keyLower] = info
				}
				info.requiredBy = append(info.requiredBy, mwItem.Name)
				if hRule.Description != "" {
					info.description = hRule.Description
				}
			}
		}
	}

	// Generate cURL command
	var curlCmd string
	if fn.ContentType == "multipart/form-data" {
		var curlLines []string
		curlLines = append(curlLines, fmt.Sprintf(`curl -X POST "%s" \`, fullRpcUrl))
		curlLines = append(curlLines, fmt.Sprintf(`  -F "namespace=%s" \`, fn.Namespace))
		curlLines = append(curlLines, fmt.Sprintf(`  -F "functionName=%s" \`, fn.FunctionName))

		if fn.InputSchema != nil && len(fn.InputSchema.Properties) > 0 {
			pKeys := make([]string, 0, len(fn.InputSchema.Properties))
			for k := range fn.InputSchema.Properties {
				pKeys = append(pKeys, k)
			}
			sort.Strings(pKeys)
			for _, k := range pKeys {
				pRule := fn.InputSchema.Properties[k]
				if pRule.Type == schema.TypeFile || pRule.Format == "binary" {
					curlLines = append(curlLines, fmt.Sprintf(`  -F "%s=@/path/to/file.jpg" \`, k))
				} else {
					curlLines = append(curlLines, fmt.Sprintf(`  -F "%s=example" \`, k))
				}
			}
		}
		// Trim trailing backslash on last item
		lastIdx := len(curlLines) - 1
		curlLines[lastIdx] = strings.TrimSuffix(curlLines[lastIdx], " \\")
		curlCmd = strings.Join(curlLines, "\n")
	} else {
		var headerLines []string
		headerLines = append(headerLines, `-H "Content-Type: application/json"`)
		hKeys := make([]string, 0, len(requiredHeadersMap))
		for k := range requiredHeadersMap {
			hKeys = append(hKeys, k)
		}
		sort.Strings(hKeys)
		for _, hk := range hKeys {
			headerLines = append(headerLines, fmt.Sprintf(`-H "%s: <YOUR_%s>"`, hk, strings.ToUpper(strings.ReplaceAll(hk, "-", "_"))))
		}
		hJoined := strings.Join(headerLines, " \\\n  ")
		curlCmd = fmt.Sprintf(`curl -X POST "%s" \
  %s \
  -d '%s'`, fullRpcUrl, hJoined, string(reqBytes))
	}

	var body strings.Builder
	body.WriteString(fmt.Sprintf(`
    <div class="page-header">
      <div class="page-breadcrumb">
        <a href="%s">← All functions</a>
        <span>/</span>
        <span>%s</span>
      </div>
      <div class="page-title">%s</div>`,
		normDocs,
		html.EscapeString(fn.Namespace),
		html.EscapeString(fn.FunctionName),
	))

	if fn.Metadata.Description != "" {
		body.WriteString(fmt.Sprintf(`<div class="page-sub">%s</div>`, html.EscapeString(fn.Metadata.Description)))
	}

	publicBadge := `<span class="badge badge-gray">PRIVATE</span>`
	if fn.Metadata.IsPublic {
		publicBadge = `<span class="badge badge-green">PUBLIC</span>`
	}
	depBadge := ""
	if fn.Metadata.Deprecated {
		depBadge = `<span class="badge badge-yellow">DEPRECATED</span>`
	}

	body.WriteString(fmt.Sprintf(`
      <div class="info-row" style="margin-top:14px">
        %s
        %s
        <span class="badge badge-purple">%s</span>
        %s
      </div>
    </div>`,
		publicBadge,
		depBadge,
		html.EscapeString(fn.ContentType),
		tagsHtml,
	))

	if fn.Metadata.Deprecated {
		body.WriteString(`<div style="background:rgba(210, 153, 34, 0.15);border:1px solid rgba(210, 153, 34, 0.3);border-radius:8px;padding:12px 16px;color:var(--yellow);margin-bottom:24px;font-size:13px">⚠️ This function is deprecated and may be removed in a future version.</div>`)
	}

	// 1. Dynamic Middleware Pipeline Section
	mwContent := `<div style="color:var(--muted);font-size:13.5px">No middlewares attached to this function. Executing directly.</div>`
	if len(middlewareCards) > 0 {
		mwContent = fmt.Sprintf(`<div class="card-stack">%s</div>`, strings.Join(middlewareCards, ""))
	}

	body.WriteString(fmt.Sprintf(`
    <div class="section">
      <div class="section-title">
        <span>Middleware Pipeline & Applied Controls (%d)</span>
      </div>
      %s
    </div>`, len(fn.Middlewares), mwContent))

	// 2. Dynamic Required Headers Section
	if len(requiredHeadersMap) > 0 {
		hKeys := make([]string, 0, len(requiredHeadersMap))
		for k := range requiredHeadersMap {
			hKeys = append(hKeys, k)
		}
		sort.Strings(hKeys)

		var headerRows strings.Builder
		for _, hk := range hKeys {
			info := requiredHeadersMap[hk]
			var badges []string
			for _, mw := range info.requiredBy {
				badges = append(badges, fmt.Sprintf(`<span class="badge badge-purple">⚡ %s</span>`, html.EscapeString(mw)))
			}
			desc := info.description
			if desc == "" {
				desc = "HTTP Header value required by middleware"
			}
			headerRows.WriteString(fmt.Sprintf(`
        <tr>
          <td class="header-key">%s</td>
          <td>%s</td>
          <td style="color:var(--muted);font-size:12.5px">%s</td>
        </tr>`, html.EscapeString(hk), strings.Join(badges, " "), html.EscapeString(desc)))
		}

		body.WriteString(fmt.Sprintf(`
      <div class="section">
        <div class="section-title">Required HTTP Headers</div>
        <div style="color:var(--muted);font-size:13px;margin-bottom:12px">
          These HTTP headers are consumed by attached middlewares before reaching your function logic.
        </div>
        <table class="header-table">
          <thead>
            <tr>
              <th>Header Key</th>
              <th>Required By</th>
              <th>Description</th>
            </tr>
          </thead>
          <tbody>
            %s
          </tbody>
        </table>
      </div>`, headerRows.String()))
	}

	// 3. Terminal cURL Command
	body.WriteString(fmt.Sprintf(`
    <div class="section">
      <div class="section-title">Terminal cURL Command</div>
      %s
    </div>`, codeBlock(curlCmd)))

	// 4. Request & Response Payload Examples
	body.WriteString(fmt.Sprintf(`
    <div class="section">
      <div class="section-title">Request Payload Example</div>
      <div style="color:var(--muted);font-size:12px;margin-bottom:8px">POST %s &nbsp;·&nbsp; Content-Type: %s</div>
      %s
    </div>

    <div class="section">
      <div class="section-title">Response Payload Example</div>
      %s
    </div>`,
		html.EscapeString(fullRpcUrl),
		html.EscapeString(fn.ContentType),
		codeBlock(string(reqBytes)),
		codeBlock(string(resBytes)),
	))

	// 5. 2-Column Split View For Input Contract
	body.WriteString(render2ColumnSchemaSection("Input Data Contract", fn.InputSchema, fn.ContentType))

	// 6. 2-Column Split View For Output Contract
	body.WriteString(render2ColumnSchemaSection("Output Data Contract", fn.OutputSchema, "application/json"))

	return htmlShell(fmt.Sprintf("%s — API Docs", fn.FunctionName), body.String(), buildSidebar(allFunctions, allMws, fn.Key, normDocs), normDocs)
}

func renderDocsNotFound(docsPath string) string {
	normDocs := normalizeDocsEndpoint(docsPath)
	body := fmt.Sprintf(`
    <div class="page-header">
      <div class="page-title">404 — Document Not Found</div>
      <div class="page-sub">The requested API document or function does not exist.</div>
    </div>
    <p><a href="%s">← Return to API Documentation Index</a></p>`, normDocs)
	return htmlShell("404 Not Found — API Docs", body, "", normDocs)
}
