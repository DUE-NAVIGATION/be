package com.due;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.boot.context.properties.ConfigurationPropertiesScan;

@SpringBootApplication
@ConfigurationPropertiesScan
public class DueBackendApplication {

	public static void main(String[] args) {
		SpringApplication.run(DueBackendApplication.class, args);
	}

}
