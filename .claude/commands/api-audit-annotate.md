---
description: Insert swagger/OpenAPI annotations into API handler source files. Run after `apiaudit scan` to add documentation to unannotated endpoints.
---

You are adding swagger/OpenAPI annotations to API handler files. The `apiaudit` CLI has already scanned the codebase and identified routes that need documentation.

## Input

Run `apiaudit scan --dir=$ARGUMENTS --format=json` to get the list of routes. If $ARGUMENTS is empty, use the current working directory.

## Process

1. Run `apiaudit scan` to get routes as JSON
2. Filter to routes where `hasSwagger` is false
3. For each unannotated route, read the handler file at the specified line
4. Determine the framework from the scan output
5. Add appropriate swagger annotations ABOVE the handler function:

### Go (swaggo style)
```go
// @Summary      Short description
// @Description  Detailed description
// @Tags         GroupName
// @Accept       json
// @Produce      json
// @Param        id path string true "Resource ID"
// @Success      200 {object} ResponseType
// @Failure      400 {object} ErrorResponse
// @Router       /path [method]
```

### Express/NestJS (JSDoc + swagger-jsdoc style)
```javascript
/**
 * @openapi
 * /path:
 *   get:
 *     summary: Short description
 *     tags: [GroupName]
 *     parameters:
 *       - in: path
 *         name: id
 *         required: true
 *         schema:
 *           type: string
 *     responses:
 *       200:
 *         description: Success
 */
```

### FastAPI/Flask (already has built-in OpenAPI for FastAPI, add docstrings for Flask)
For Flask, add flasgger-style docstrings.
For FastAPI, add response_model and description params if missing.

## Rules
- Read the handler function to understand request/response shapes
- Use the actual parameter names from the route path
- Infer response types from return statements
- Group by URL prefix for tags
- Ask the user before writing any changes: show a summary of what will be annotated
- Do NOT modify any logic — only add documentation annotations
