package com.example.aop;

import io.micronaut.aop.Around;
import io.micronaut.aop.InterceptorBean;
import io.micronaut.aop.MethodInterceptor;
import io.micronaut.aop.MethodInvocationContext;
import jakarta.inject.Singleton;
import java.lang.annotation.*;

/**
 * Micronaut AOP example: logging interceptor.
 *
 * @Logged is the binding annotation (acts as pointcut designator).
 * LoggingInterceptor implements MethodInterceptor and is bound via @InterceptorBean.
 */

@Around
@Retention(RetentionPolicy.RUNTIME)
@Target({ElementType.METHOD, ElementType.TYPE})
public @interface Logged {
    boolean includeArgs() default true;
}

@Singleton
@InterceptorBean(Logged.class)
public class LoggingInterceptor implements MethodInterceptor<Object, Object> {

    @Override
    public Object intercept(MethodInvocationContext<Object, Object> context) {
        System.out.println("BEFORE " + context.getMethodName());
        try {
            return context.proceed();
        } finally {
            System.out.println("AFTER " + context.getMethodName());
        }
    }
}
