// Package logsapi implements logs receiving HTTP server and decoding function to use Lambda Logs API.
// Implement Processor and use Run function in your main package.
// For more custom use cases you can use low-level DecodeLogs function directly.
//
// Deprecated: The Lambda Telemetry API supersedes the Lambda Logs API.
// While the Logs API remains fully functional, we recommend using only the Telemetry API going forward.
// Use telemetryapi.Run instead.
// https://docs.aws.amazon.com/lambda/latest/dg/runtimes-logs-api.html
// https://aws.amazon.com/blogs/compute/introducing-the-aws-lambda-telemetry-api/
package logsapi
