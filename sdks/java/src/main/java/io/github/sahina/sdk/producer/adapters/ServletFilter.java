package io.github.sahina.sdk.producer.adapters;

import io.github.sahina.sdk.producer.Producer;
import io.github.sahina.sdk.producer.ProducerConfig;
import io.github.sahina.sdk.producer.ProducerValidationResult;
import io.github.sahina.sdk.producer.ValidationMode;
import com.google.gson.Gson;

import jakarta.servlet.Filter;
import jakarta.servlet.FilterChain;
import jakarta.servlet.FilterConfig;
import jakarta.servlet.ServletException;
import jakarta.servlet.ServletRequest;
import jakarta.servlet.ServletResponse;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;

import java.io.BufferedReader;
import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.PrintWriter;
import java.nio.charset.StandardCharsets;
import java.util.Collections;
import java.util.Enumeration;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import java.util.logging.Level;
import java.util.logging.Logger;

/**
 * Servlet Filter for producer validation.
 * Works with Jakarta Servlet API (Tomcat 10+, Jetty 11+, Spring 6+).
 *
 * <p>Example (Spring Boot):
 * <pre>{@code
 * @Bean
 * public FilterRegistrationBean<ServletFilter> producerValidationFilter() {
 *     FilterRegistrationBean<ServletFilter> registration = new FilterRegistrationBean<>();
 *     registration.setFilter(new ServletFilter(producerConfig));
 *     registration.addUrlPatterns("/api/*");
 *     registration.setOrder(1);
 *     return registration;
 * }
 * }</pre>
 */
public class ServletFilter implements Filter {
    private static final Logger LOGGER = Logger.getLogger(ServletFilter.class.getName());
    private static final Gson GSON = new Gson();

    private final Producer producer;
    private final ProducerConfig config;

    /**
     * Creates a new ServletFilter with the given configuration.
     *
     * @param config The producer configuration
     */
    public ServletFilter(ProducerConfig config) {
        this.config = config;
        this.producer = new Producer(config);
    }

    @Override
    public void init(FilterConfig filterConfig) throws ServletException {
        // No initialization needed
    }

    @Override
    public void doFilter(ServletRequest request, ServletResponse response, FilterChain chain)
            throws IOException, ServletException {

        if (!(request instanceof HttpServletRequest) || !(response instanceof HttpServletResponse)) {
            chain.doFilter(request, response);
            return;
        }

        HttpServletRequest httpRequest = (HttpServletRequest) request;
        HttpServletResponse httpResponse = (HttpServletResponse) response;

        // Get path for filtering
        String path = httpRequest.getRequestURI();
        String queryString = httpRequest.getQueryString();
        if (queryString != null && !queryString.isEmpty()) {
            path = path + "?" + queryString;
        }

        // Check path filters
        if (!producer.shouldValidatePath(path)) {
            chain.doFilter(request, response);
            return;
        }

        String method = httpRequest.getMethod();
        Map<String, String> headers = extractHeaders(httpRequest);

        // Read request body
        CachedBodyHttpServletRequest wrappedRequest = new CachedBodyHttpServletRequest(httpRequest);
        String requestBody = wrappedRequest.getBody();

        // Shadow mode: async validation
        if (config.getMode() == ValidationMode.SHADOW) {
            if (config.isValidateRequest()) {
                final String finalPath = path;
                CompletableFuture.runAsync(() -> {
                    try {
                        ProducerValidationResult result = producer.validateRequest(
                                method, finalPath, headers, requestBody);
                        if (!result.isValid()) {
                            producer.handleRequestFailure(result, httpRequest);
                        }
                    } catch (Exception e) {
                        LOGGER.log(Level.WARNING, "[" + config.getLogPrefix() + "] Shadow validation error", e);
                    }
                });
            }

            // Capture response for validation
            CachedBodyHttpServletResponse wrappedResponse = new CachedBodyHttpServletResponse(httpResponse);
            chain.doFilter(wrappedRequest, wrappedResponse);

            if (config.isValidateResponse()) {
                String responseBody = wrappedResponse.getBody();
                int statusCode = wrappedResponse.getStatus();
                Map<String, String> responseHeaders = extractResponseHeaders(wrappedResponse);
                final String finalPath2 = path;

                CompletableFuture.runAsync(() -> {
                    try {
                        ProducerValidationResult result = producer.validateResponse(
                                method, finalPath2, headers, requestBody,
                                statusCode, responseHeaders, responseBody);
                        if (!result.isValid()) {
                            producer.handleResponseFailure(result, httpRequest, httpResponse);
                        }
                    } catch (Exception e) {
                        LOGGER.log(Level.WARNING, "[" + config.getLogPrefix() + "] Shadow validation error", e);
                    }
                });
            }
            return;
        }

        // Strict/Warn mode: validate request first
        if (config.isValidateRequest()) {
            try {
                ProducerValidationResult result = producer.validateRequest(
                        method, path, headers, requestBody);

                if (!result.isValid()) {
                    Object[] handlerResult = producer.handleRequestFailure(result, httpRequest);
                    boolean shouldContinue = (Boolean) handlerResult[0];
                    Object customResponse = handlerResult[1];

                    if (!shouldContinue) {
                        sendErrorResponse(httpResponse, result, customResponse);
                        return;
                    }
                }
            } catch (Exception e) {
                LOGGER.log(Level.WARNING, "[" + config.getLogPrefix() + "] Request validation error", e);
                // Continue on error
            }
        }

        // Capture response
        CachedBodyHttpServletResponse wrappedResponse = new CachedBodyHttpServletResponse(httpResponse);
        chain.doFilter(wrappedRequest, wrappedResponse);

        // Validate response
        if (config.isValidateResponse()) {
            try {
                String responseBody = wrappedResponse.getBody();
                int statusCode = wrappedResponse.getStatus();
                Map<String, String> responseHeaders = extractResponseHeaders(wrappedResponse);

                ProducerValidationResult result = producer.validateResponse(
                        method, path, headers, requestBody,
                        statusCode, responseHeaders, responseBody);

                if (!result.isValid()) {
                    producer.handleResponseFailure(result, httpRequest, httpResponse);
                }
            } catch (Exception e) {
                LOGGER.log(Level.WARNING, "[" + config.getLogPrefix() + "] Response validation error", e);
            }
        }
    }

    @Override
    public void destroy() {
        // No cleanup needed
    }

    private Map<String, String> extractHeaders(HttpServletRequest request) {
        Map<String, String> headers = new HashMap<>();
        Enumeration<String> headerNames = request.getHeaderNames();
        while (headerNames.hasMoreElements()) {
            String name = headerNames.nextElement();
            headers.put(name.toLowerCase(), request.getHeader(name));
        }
        return headers;
    }

    private Map<String, String> extractResponseHeaders(HttpServletResponse response) {
        Map<String, String> headers = new HashMap<>();
        for (String name : response.getHeaderNames()) {
            headers.put(name.toLowerCase(), response.getHeader(name));
        }
        return headers;
    }

    private void sendErrorResponse(HttpServletResponse response,
                                   ProducerValidationResult result,
                                   Object customResponse) throws IOException {
        response.setContentType("application/json");
        response.setStatus(HttpServletResponse.SC_BAD_REQUEST);

        Map<String, Object> errorBody;
        if (customResponse != null) {
            if (customResponse instanceof Map) {
                errorBody = (Map<String, Object>) customResponse;
            } else {
                errorBody = new HashMap<>();
                errorBody.put("error", customResponse.toString());
            }
        } else {
            errorBody = new HashMap<>();
            errorBody.put("error", "Request validation failed");
            errorBody.put("details", result.getErrors());
        }

        response.getWriter().write(GSON.toJson(errorBody));
    }

    /**
     * Wrapper for HttpServletRequest that caches the body for re-reading.
     */
    private static class CachedBodyHttpServletRequest extends jakarta.servlet.http.HttpServletRequestWrapper {
        private final byte[] cachedBody;

        public CachedBodyHttpServletRequest(HttpServletRequest request) throws IOException {
            super(request);
            InputStream inputStream = request.getInputStream();
            this.cachedBody = inputStream.readAllBytes();
        }

        @Override
        public jakarta.servlet.ServletInputStream getInputStream() {
            return new CachedServletInputStream(cachedBody);
        }

        @Override
        public BufferedReader getReader() {
            return new BufferedReader(new InputStreamReader(
                    new ByteArrayInputStream(cachedBody), StandardCharsets.UTF_8));
        }

        public String getBody() {
            return new String(cachedBody, StandardCharsets.UTF_8);
        }
    }

    /**
     * Cached ServletInputStream implementation.
     */
    private static class CachedServletInputStream extends jakarta.servlet.ServletInputStream {
        private final ByteArrayInputStream inputStream;

        public CachedServletInputStream(byte[] cachedBody) {
            this.inputStream = new ByteArrayInputStream(cachedBody);
        }

        @Override
        public int read() {
            return inputStream.read();
        }

        @Override
        public boolean isFinished() {
            return inputStream.available() == 0;
        }

        @Override
        public boolean isReady() {
            return true;
        }

        @Override
        public void setReadListener(jakarta.servlet.ReadListener listener) {
            throw new UnsupportedOperationException();
        }
    }

    /**
     * Wrapper for HttpServletResponse that captures the response body.
     */
    private static class CachedBodyHttpServletResponse extends jakarta.servlet.http.HttpServletResponseWrapper {
        private final ByteArrayOutputStream cachedBody = new ByteArrayOutputStream();
        private final PrintWriter writer;
        private final jakarta.servlet.ServletOutputStream outputStream;

        public CachedBodyHttpServletResponse(HttpServletResponse response) throws IOException {
            super(response);
            this.outputStream = new CachedServletOutputStream(response.getOutputStream(), cachedBody);
            this.writer = new PrintWriter(cachedBody, true, StandardCharsets.UTF_8);
        }

        @Override
        public PrintWriter getWriter() {
            return writer;
        }

        @Override
        public jakarta.servlet.ServletOutputStream getOutputStream() {
            return outputStream;
        }

        @Override
        public void flushBuffer() throws IOException {
            writer.flush();
            super.flushBuffer();
        }

        public String getBody() {
            return cachedBody.toString(StandardCharsets.UTF_8);
        }
    }

    /**
     * Cached ServletOutputStream implementation.
     */
    private static class CachedServletOutputStream extends jakarta.servlet.ServletOutputStream {
        private final jakarta.servlet.ServletOutputStream original;
        private final ByteArrayOutputStream cache;

        public CachedServletOutputStream(jakarta.servlet.ServletOutputStream original, ByteArrayOutputStream cache) {
            this.original = original;
            this.cache = cache;
        }

        @Override
        public void write(int b) throws IOException {
            original.write(b);
            cache.write(b);
        }

        @Override
        public void write(byte[] b) throws IOException {
            original.write(b);
            cache.write(b);
        }

        @Override
        public void write(byte[] b, int off, int len) throws IOException {
            original.write(b, off, len);
            cache.write(b, off, len);
        }

        @Override
        public boolean isReady() {
            return original.isReady();
        }

        @Override
        public void setWriteListener(jakarta.servlet.WriteListener listener) {
            original.setWriteListener(listener);
        }
    }
}
