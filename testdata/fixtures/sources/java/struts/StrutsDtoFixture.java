package com.example.struts;

import org.apache.struts.action.ActionForm;
import org.apache.struts.validator.ValidatorForm;
import org.apache.struts2.convention.annotation.Action;
import com.opensymphony.xwork2.ActionSupport;
import com.opensymphony.xwork2.ModelDriven;

// ── Struts 1: classic ActionForm subclass ───────────────────────────────────
// The form is the request-backing DTO; its setters are the bound fields.
public class UserForm extends ActionForm {
    private String username;
    private String email;
    private int age;

    public String getUsername() { return username; }
    public void setUsername(String username) { this.username = username; }

    public String getEmail() { return email; }
    public void setEmail(String email) { this.email = email; }

    public int getAge() { return age; }
    public void setAge(int age) { this.age = age; }

    // Framework plumbing — must NOT be treated as a bound DTO field.
    public void setServletRequest(javax.servlet.http.HttpServletRequest request) { }
}

// ── Struts 1: ValidatorForm variant ─────────────────────────────────────────
class LoginForm extends ValidatorForm {
    private String password;
    public void setPassword(String password) { this.password = password; }
}

// ── Struts 2: ActionSupport with OGNL field binding ─────────────────────────
class OrderAction extends ActionSupport {
    private String orderId;
    private double amount;

    @Action(value = "/orders/place")
    public String place() { return SUCCESS; }

    public void setOrderId(String orderId) { this.orderId = orderId; }
    public void setAmount(double amount) { this.amount = amount; }
}

// ── Struts 2: ModelDriven exposes a separate domain model ────────────────────
class ProductAction extends ActionSupport implements ModelDriven<Product> {
    private final Product product = new Product();

    public Product getModel() { return product; }

    public void setCategory(String category) { }
}

class Product {
    private String sku;
    public void setSku(String sku) { this.sku = sku; }
}
