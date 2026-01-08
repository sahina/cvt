package com.cvt.sdk.producer.adapters;

import com.cvt.sdk.producer.Producer;
import com.cvt.sdk.producer.ProducerConfig;
import com.cvt.sdk.producer.ProducerValidationResult;
import com.cvt.sdk.producer.ValidationMode;
import com.google.gson.Gson;

import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;

import org.springframework.web.servlet.HandlerInterceptor;
import org.springframework.web.servlet.ModelAndView;
import org.springframework.web.util.ContentCachingRequestWrapper;
import org.springframework.web.util.ContentCachingResponseWrapper;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.Enumeration;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import java.util.logging.Level;
import java.util.logging.Logger;

/**
 * Spring MVC HandlerInterceptor for producer validation.
 * Works with Spring Framework 6+ and Spring Boot 3+.
 *
 * <p>Example (Spring Boot):
 * <pre>{@code
 * @Configuration
 * public class WebConfig implements WebMvcConfigurer {
 *     @Autowired
 *     private ProducerConfig producerConfig;
 *
 *     @Override
 *     public void addInterceptors(InterceptorRegistry registry) {
 *         registry.addInterceptor(new SpringInterceptor(producerConfig))
 *                 .addPathPatterns("/api/**");
 *     }
 * }
 * }</pre>
 *
 * <p>Note: You must also register a ContentCachingRequestWrapper filter to enable
 * request body caching:
 * <pre>{@code
 * @Bean
 * public FilterRegistrationBean<ContentCachingFilter> contentCachingFilter() {
 *     FilterRegistrationBean<ContentCachingFilter> registration = new FilterRegistrationBean<>();
 *     registration.setFilter(new ContentCachingFilter());
 *     registration.addUrlPatterns("/api/*");
 *     registration.setOrder(Ordered.HIGHEST_PRECEDENCE);
 *     return registration;
 * }
 * }</pre>
 */
public class SpringInterceptor implements HandlerInterceptor {
    private static final Logger LOGGER = Logger.getLogger(SpringInterceptor.class.getName());
    private static final Gson GSON = new Gson();
    private static final String REQUEST_BODY_ATTR = "cvt.requestBody";
    private static final String REQUEST_HEADERS_ATTR = "cvt.requestHeaders";

    private final Producer producer;
    private final ProducerConfig config;

    /**
     * Creates a new SpringInterceptor with the given configuration.
     *
     * @param config The producer configuration
     */
    public SpringInterceptor(ProducerConfig config) {
        this.config = config;
        this.producer = new Producer(config);
    }

    @Override
    public boolean preHandle(HttpServletRequest request, HttpServletResponse response, Object handler)
            throws Exception {

        // Get path for filtering
        String path = request.getRequestURI();
        String queryString = request.getQueryString();
        if (queryString != null && !queryString.isEmpty()) {
            path = path + "?" + queryString;
        }

        // Check path filters
        if (!producer.shouldValidatePath(path)) {
            return true;
        }

        String method = request.getMethod();
        Map<String, String> headers = extractHeaders(request);

        // Store headers for later use
        request.setAttribute(REQUEST_HEADERS_ATTR, headers);

        // Get request body from ContentCachingRequestWrapper
        String requestBody = null;
        if (request instanceof ContentCachingRequestWrapper) {
            byte[] content = ((ContentCachingRequestWrapper) request).getContentAsByteArray();
            if (content.length > 0) {
                requestBody = new String(content, StandardCharsets.UTF_8);
            }
        }
        request.setAttribute(REQUEST_BODY_ATTR, requestBody);

        // Skip request validation if disabled
        if (!config.isValidateRequest()) {
            return true;
        }

        // Shadow mode: async validation
        if (config.getMode() == ValidationMode.SHADOW) {
            final String finalPath = path;
            final String finalBody = requestBody;
            CompletableFuture.runAsync(() -> {
                try {
                    ProducerValidationResult result = producer.validateRequest(
                            method, finalPath, headers, finalBody);
                    if (!result.isValid()) {
                        producer.handleRequestFailure(result, request);
                    }
                } catch (Exception e) {
                    LOGGER.log(Level.WARNING, "[" + config.getLogPrefix() + "] Shadow validation error", e);
                }
            });
            return true;
        }

        // Strict/Warn mode: validate request
        try {
            ProducerValidationResult result = producer.validateRequest(
                    method, path, headers, requestBody);

            if (!result.isValid()) {
                Object[] handlerResult = producer.handleRequestFailure(result, request);
                boolean shouldContinue = (Boolean) handlerResult[0];
                Object customResponse = handlerResult[1];

                if (!shouldContinue) {
                    sendErrorResponse(response, result, customResponse);
                    return false;
                }
            }
        } catch (Exception e) {
            LOGGER.log(Level.WARNING, "[" + config.getLogPrefix() + "] Request validation error", e);
            // Continue on error
        }

        return true;
    }

    @Override
    public void postHandle(HttpServletRequest request, HttpServletResponse response,
                           Object handler, ModelAndView modelAndView) {
        // No action needed here
    }

    @Override
    public void afterCompletion(HttpServletRequest request, HttpServletResponse response,
                                Object handler, Exception ex) {

        if (!config.isValidateResponse()) {
            return;
        }

        // Get path for filtering
        String path = request.getRequestURI();
        String queryString = request.getQueryString();
        if (queryString != null && !queryString.isEmpty()) {
            path = path + "?" + queryString;
        }

        // Check path filters
        if (!producer.shouldValidatePath(path)) {
            return;
        }

        String method = request.getMethod();

        // Get stored request data
        @SuppressWarnings("unchecked")
        Map<String, String> requestHeaders = (Map<String, String>) request.getAttribute(REQUEST_HEADERS_ATTR);
        String requestBody = (String) request.getAttribute(REQUEST_BODY_ATTR);

        // Get response data
        String responseBody = null;
        int statusCode = response.getStatus();
        Map<String, String> responseHeaders = extractResponseHeaders(response);

        if (response instanceof ContentCachingResponseWrapper) {
            byte[] content = ((ContentCachingResponseWrapper) response).getContentAsByteArray();
            if (content.length > 0) {
                responseBody = new String(content, StandardCharsets.UTF_8);
            }
        }

        // Shadow mode: async validation
        if (config.getMode() == ValidationMode.SHADOW) {
            final String finalPath = path;
            final String finalResponseBody = responseBody;
            CompletableFuture.runAsync(() -> {
                try {
                    ProducerValidationResult result = producer.validateResponse(
                            method, finalPath, requestHeaders, requestBody,
                            statusCode, responseHeaders, finalResponseBody);
                    if (!result.isValid()) {
                        producer.handleResponseFailure(result, request, response);
                    }
                } catch (Exception e) {
                    LOGGER.log(Level.WARNING, "[" + config.getLogPrefix() + "] Shadow validation error", e);
                }
            });
            return;
        }

        // Strict/Warn mode: validate response
        try {
            ProducerValidationResult result = producer.validateResponse(
                    method, path, requestHeaders, requestBody,
                    statusCode, responseHeaders, responseBody);

            if (!result.isValid()) {
                producer.handleResponseFailure(result, request, response);
            }
        } catch (Exception e) {
            LOGGER.log(Level.WARNING, "[" + config.getLogPrefix() + "] Response validation error", e);
        }
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
                @SuppressWarnings("unchecked")
                Map<String, Object> map = (Map<String, Object>) customResponse;
                errorBody = map;
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
}
